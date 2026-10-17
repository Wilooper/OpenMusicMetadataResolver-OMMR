package providerhealth

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestCircuitSuspendsAndAllowsOnlyOneRecoveryProbe(t *testing.T) {
	now := time.Now()
	m := New(2, time.Minute)
	m.now = func() time.Time { return now }
	m.Set("test", "active", "public", "absent", "")
	initial := m.Signature()
	lease, _ := m.Begin("test")
	late, _ := m.Begin("test")
	m.Finish(context.Background(), "test", lease, errors.New("HTTP 500"))
	lease, _ = m.Begin("test")
	m.Finish(context.Background(), "test", lease, errors.New("HTTP 500"))
	if m.Available("test") || m.Signature() == initial {
		t.Fatal("circuit did not suspend")
	}
	m.Finish(context.Background(), "test", late, nil)
	if m.Available("test") {
		t.Fatal("stale in-flight success reopened the circuit")
	}
	now = now.Add(time.Minute)
	if !m.Available("test") {
		t.Fatal("recovery never becomes eligible")
	}
	lease, err := m.Begin("test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Begin("test"); !errors.Is(err, ErrSuspended) {
		t.Fatal("more than one recovery probe was admitted")
	}
	m.Finish(context.Background(), "test", lease, nil)
	if !m.Available("test") || m.Snapshot()[0].ConsecutiveFailures != 0 {
		t.Fatal("recovery did not reset failures")
	}
}
func TestCancellationMissingMetadataAndDisabledStates(t *testing.T) {
	m := New(1, time.Minute)
	m.Set("test", "active", "public", "absent", "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	lease, _ := m.Begin("test")
	m.Finish(ctx, "test", lease, ctx.Err())
	lease, _ = m.Begin("test")
	m.Finish(context.Background(), "test", lease, errors.New("catalog HTTP 404"))
	if !m.Available("test") {
		t.Fatal("cancellation or missing artwork suspended source")
	}
	m.Set("missing", "disabled", "off", "absent", "credentials_missing")
	if m.Available("missing") {
		t.Fatal("missing credentials were made available")
	}
}
func TestCredentialURLsAreNeverReturned(t *testing.T) {
	err := sanitize(&url.Error{Op: "Get", URL: "https://provider.invalid/?api_key=private", Err: errors.New("failed")})
	if strings.Contains(err.Error(), "private") {
		t.Fatal("credential URL leaked")
	}
}
