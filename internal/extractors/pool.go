package extractors

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/ommr/ommr/internal/cache"
	"golang.org/x/sync/singleflight"
)

const maxResponse = 4 << 20

var ErrBusy = errors.New("extractor queue is full")

type Config struct {
	Python, Script string
	Workers        int
	Timeout, TTL   time.Duration
}
type worker struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	reader *bufio.Reader
}
type Pool struct {
	cfg     Config
	ctx     context.Context
	cancel  context.CancelFunc
	slots   chan *worker
	queue   chan struct{}
	cache   cache.Cache
	flights singleflight.Group
	close   sync.Once
	mu      sync.Mutex
	closed  bool
}
type reply struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Partial bool            `json:"partial"`
}

func New(cfg Config, backend cache.Cache) (*Pool, error) {
	if cfg.Workers < 1 || cfg.Workers > 8 || cfg.Timeout <= 0 || cfg.TTL <= 0 || cfg.Python == "" || cfg.Script == "" {
		return nil, errors.New("invalid extractor configuration")
	}
	ctx, cancel := context.WithCancel(context.Background())
	p := &Pool{cfg: cfg, ctx: ctx, cancel: cancel, slots: make(chan *worker, cfg.Workers), queue: make(chan struct{}, cfg.Workers*4), cache: backend}
	for range cfg.Workers {
		p.slots <- nil
	}
	probe, stop := context.WithTimeout(ctx, cfg.Timeout)
	defer stop()
	if _, err := p.Call(probe, "ping", "", 10); err != nil {
		p.Close()
		return nil, errors.New("Python extractor startup failed; check executable, dependencies and script")
	}
	return p, nil
}
func (p *Pool) Close() {
	p.close.Do(func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.closed = true
		p.cancel()
		for {
			select {
			case w := <-p.slots:
				stopWorker(w)
			default:
				return
			}
		}
	})
}
func stopWorker(w *worker) {
	if w != nil {
		_ = w.stdin.Close()
		_ = w.cmd.Process.Kill()
		_ = w.cmd.Wait()
	}
}
func (p *Pool) start() (*worker, error) {
	cmd := exec.CommandContext(p.ctx, p.cfg.Python, "-u", p.cfg.Script)
	input, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		_ = input.Close()
		return nil, err
	}
	// Credentials and upstream exceptions must never reach the service log.
	cmd.Stderr = io.Discard
	if err = cmd.Start(); err != nil {
		_ = input.Close()
		return nil, err
	}
	return &worker{cmd, input, bufio.NewReaderSize(output, 64<<10)}, nil
}
func (p *Pool) Call(ctx context.Context, operation, id string, limit int) (json.RawMessage, error) {
	return p.call(ctx, operation, id, limit, false)
}

func (p *Pool) CallFresh(ctx context.Context, operation, id string, limit int) (json.RawMessage, error) {
	return p.call(ctx, operation, id, limit, true)
}

func (p *Pool) call(ctx context.Context, operation, id string, limit int, bypass bool) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p.ctx.Err() != nil {
		return nil, errors.New("extractor closed")
	}
	if operation == "song" {
		limit = 1
	}
	body, _ := json.Marshal(map[string]any{"operation": operation, "id": id, "limit": limit})
	sum := sha256.Sum256(body)
	key := "extractor:ytmusic-v1:" + hex.EncodeToString(sum[:])
	if !bypass && operation != "ping" && p.cache != nil {
		if b, ok, err := p.cache.Get(ctx, key); err == nil && ok && json.Valid(b) {
			return append(json.RawMessage(nil), b...), nil
		}
	}
	flightKey := key
	if bypass {
		flightKey += ":fresh"
	}
	result := p.flights.DoChan(flightKey, func() (any, error) {
		select {
		case p.queue <- struct{}{}:
			defer func() { <-p.queue }()
		default:
			return nil, ErrBusy
		}
		work, cancel := context.WithTimeout(p.ctx, p.cfg.Timeout)
		defer cancel()
		data, partial, err := p.request(work, body)
		if err == nil && partial {
			var fields map[string]json.RawMessage
			if json.Unmarshal(data, &fields) == nil && fields != nil {
				fields["partial"] = json.RawMessage(`true`)
				data, _ = json.Marshal(fields)
			}
		}
		if err == nil && p.cache != nil && operation != "ping" {
			ttl := p.cfg.TTL
			if partial && ttl > 30*time.Second {
				ttl = 30 * time.Second
			}
			_ = p.cache.Set(work, key, data, ttl)
		}
		return data, err
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response := <-result:
		if response.Err != nil {
			return nil, response.Err
		}
		return append(json.RawMessage(nil), response.Val.(json.RawMessage)...), nil
	}
}
func (p *Pool) request(ctx context.Context, body []byte) (json.RawMessage, bool, error) {
	var w *worker
	select {
	case w = <-p.slots:
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
	healthy := false
	defer func() {
		if !healthy {
			stopWorker(w)
			w = nil
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		if p.closed {
			stopWorker(w)
			return
		}
		p.slots <- w
	}()
	if w == nil {
		var err error
		w, err = p.start()
		if err != nil {
			return nil, false, errors.New("extractor process unavailable")
		}
	}
	completed := make(chan struct {
		data              json.RawMessage
		err               error
		partial, reusable bool
	}, 1)
	go func() {
		var data json.RawMessage
		var partial, reusable bool
		_, err := w.stdin.Write(append(body, '\n'))
		if err == nil {
			var line []byte
			for {
				part, more, e := w.reader.ReadLine()
				if e != nil {
					err = e
					break
				}
				if len(line)+len(part) > maxResponse {
					err = errors.New("extractor response exceeds limit")
					break
				}
				line = append(line, part...)
				if !more {
					break
				}
			}
			if err == nil {
				var r reply
				if json.Unmarshal(line, &r) != nil {
					err = errors.New("invalid extractor protocol")
				} else if !r.OK {
					reusable = true
					err = fmt.Errorf("extractor %s", safeError(r.Error))
				} else if len(r.Data) == 0 || !json.Valid(r.Data) || !bytes.HasPrefix(bytes.TrimSpace(r.Data), []byte("{")) {
					err = errors.New("empty extractor response")
				} else {
					data = r.Data
					partial = r.Partial
					reusable = true
				}
			}
		}
		completed <- struct {
			data              json.RawMessage
			err               error
			partial, reusable bool
		}{data, err, partial, reusable}
	}()
	select {
	case result := <-completed:
		healthy = result.reusable
		return result.data, result.partial, result.err
	case <-ctx.Done():
		_ = w.cmd.Process.Kill()
		<-completed
		return nil, false, ctx.Err()
	}
}
func safeError(code string) string {
	switch code {
	case "not_found":
		return "not found"
	case "unsupported", "invalid_id", "upstream_unavailable":
		return code
	default:
		return "unavailable"
	}
}
