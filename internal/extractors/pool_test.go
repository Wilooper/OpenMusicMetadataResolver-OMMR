package extractors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ommr/ommr/internal/cache/disk"
)

func fixturePool(t *testing.T, timeout time.Duration) *Pool {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	script := filepath.Join(t.TempDir(), "worker.py")
	if err = os.WriteFile(script, []byte(`import json,os,sys,time
counts={}
for line in sys.stdin:
 r=json.loads(line);id=r['id'];counts[id]=counts.get(id,0)+1
 if id=='hang':time.sleep(10)
 if id=='large':print('x'*(4*1024*1024+1),flush=True);continue
 if id=='error':print(json.dumps({'ok':False,'error':'SECRET_COOKIE'}),flush=True);continue
 if id.startswith('slow'):time.sleep(0.1)
 print(json.dumps({'ok':True,'data':{'pid':os.getpid(),'count':counts[id],'id':id},'partial':id=='partial'}),flush=True)
`), 0600); err != nil {
		t.Fatal(err)
	}
	backend, err := disk.NewDiskCache(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Close() })
	p, err := New(Config{Python: python, Script: script, Workers: 2, Timeout: timeout, TTL: time.Hour}, backend)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}
func TestPersistentWorkersCoalesceAndCache(t *testing.T) {
	p := fixturePool(t, time.Second)
	results := make([]json.RawMessage, 16)
	errs := make([]error, 16)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func() { defer wg.Done(); results[i], errs[i] = p.Call(context.Background(), "song", "slow-same", 20) }()
	}
	wg.Wait()
	for i, result := range results {
		if errs[i] != nil || string(result) != string(results[0]) {
			t.Fatalf("coalescing: %s %v", result, errs[i])
		}
	}
	cached, err := p.Call(context.Background(), "song", "slow-same", 20)
	if err != nil || string(cached) != string(results[0]) {
		t.Fatalf("cache failed: %v", err)
	}
	if _, err = p.Call(context.Background(), "song", "error", 20); err == nil || err.Error() != "extractor unavailable" {
		t.Fatalf("private error leaked: %v", err)
	}
}
func TestWorkerTimeoutAndMalformedOutputRecover(t *testing.T) {
	p := fixturePool(t, 250*time.Millisecond)
	for _, id := range []string{"hang", "large"} {
		if _, err := p.Call(context.Background(), "song", id, 20); err == nil {
			t.Fatal("failed worker accepted")
		}
		if _, err := p.Call(context.Background(), "song", "recovered-"+id, 20); err != nil {
			t.Fatal(err)
		}
	}
	p.Close()
	if _, err := p.Call(context.Background(), "song", "closed", 20); err == nil {
		t.Fatal("closed worker accepted work")
	}
}
func TestCanceledWaiterAndQueueCapacity(t *testing.T) {
	p := fixturePool(t, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Call(ctx, "song", "unused", 20); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// Distinct simultaneous misses must shed excess load instead of spawning processes.
	const requests = 32
	var wg sync.WaitGroup
	var mu sync.Mutex
	busy := 0
	pids := map[float64]bool{}
	for i := range requests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			data, err := p.Call(context.Background(), "song", fmt.Sprintf("slow-%d", i), 20)
			mu.Lock()
			defer mu.Unlock()
			if errors.Is(err, ErrBusy) {
				busy++
				return
			}
			if err != nil {
				t.Error(err)
				return
			}
			var result map[string]any
			_ = json.Unmarshal(data, &result)
			pids[result["pid"].(float64)] = true
		}()
	}
	wg.Wait()
	if len(pids) > 2 {
		t.Fatal("worker process budget exceeded")
	}
	if busy == 0 {
		t.Fatal("queue did not shed excess load")
	}
}

func TestCancelingAnInflightWaiterDoesNotCorruptSharedWork(t *testing.T) {
	p := fixturePool(t, time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if _, err := p.Call(ctx, "song", "slow-canceled", 20); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiter cancellation: %v", err)
	}
	data, err := p.Call(context.Background(), "song", "slow-canceled", 20)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	if result["count"] != float64(1) {
		t.Fatal("canceling waiter restarted shared work")
	}
}

type ttlCache struct{ ttl time.Duration }

func (*ttlCache) Get(context.Context, string) ([]byte, bool, error) { return nil, false, nil }
func (c *ttlCache) Set(_ context.Context, _ string, _ []byte, ttl time.Duration) error {
	c.ttl = ttl
	return nil
}
func (*ttlCache) Delete(context.Context, string) error { return nil }
func (*ttlCache) Close() error                         { return nil }
func TestPartialWorkerDataUsesShortCacheTTL(t *testing.T) {
	p := fixturePool(t, time.Second)
	backend := &ttlCache{}
	p.cache = backend
	data, err := p.Call(context.Background(), "song", "partial", 20)
	if err != nil || backend.ttl != 30*time.Second {
		t.Fatalf("partial TTL: %s %v %v", data, backend.ttl, err)
	}
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	if result["partial"] != true {
		t.Fatal("partial entity not marked")
	}
}

func TestFreshCallBypassesCachedWorkerData(t *testing.T) {
	p := fixturePool(t, time.Second)
	first, err := p.Call(context.Background(), "song", "fresh", 20)
	if err != nil {
		t.Fatal(err)
	}
	cached, err := p.Call(context.Background(), "song", "fresh", 20)
	if err != nil || string(first) != string(cached) {
		t.Fatal("cache miss")
	}
	fresh, err := p.CallFresh(context.Background(), "song", "fresh", 20)
	if err != nil || string(first) == string(fresh) {
		t.Fatal("fresh call reused cached response")
	}
	after, err := p.Call(context.Background(), "song", "fresh", 20)
	if err != nil || string(after) != string(fresh) {
		t.Fatal("fresh response did not refresh cache")
	}
}
