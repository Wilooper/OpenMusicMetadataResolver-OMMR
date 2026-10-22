package providerhealth

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrSuspended = errors.New("provider temporarily unavailable")

type Status struct {
	Name                string    `json:"name"`
	State               string    `json:"state"`
	Mode                string    `json:"mode"`
	Credentials         string    `json:"credentials"`
	Reason              string    `json:"reason,omitempty"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	LastChecked         time.Time `json:"last_checked,omitempty"`
	RetryAt             time.Time `json:"retry_at,omitempty"`
	probing             bool
	generation          uint64
}
type Manager struct {
	mu        sync.Mutex
	entries   map[string]*Status
	threshold int
	cooldown  time.Duration
	now       func() time.Time
}

func New(threshold int, cooldown time.Duration) *Manager {
	if threshold < 1 {
		threshold = 3
	}
	return &Manager{entries: map[string]*Status{}, threshold: threshold, cooldown: cooldown, now: time.Now}
}
func (m *Manager) Set(name, state, mode, credentials, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries[name] = &Status{Name: name, State: state, Mode: mode, Credentials: credentials, Reason: reason, LastChecked: m.now(), generation: 1}
}
func (m *Manager) Available(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.entries[name]
	if !ok {
		return true
	}
	return s.State == "active" || (s.State == "suspended" && m.cooldown > 0 && !m.now().Before(s.RetryAt) && !s.probing)
}
func (m *Manager) Begin(name string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.entries[name]
	if !ok {
		return 0, nil
	}
	if s.State == "active" {
		return s.generation, nil
	}
	if s.State == "suspended" && m.cooldown > 0 && !m.now().Before(s.RetryAt) && !s.probing {
		s.probing = true
		return s.generation, nil
	}
	return 0, ErrSuspended
}
func (m *Manager) Skip(name string, generation uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s := m.entries[name]; s != nil && s.generation == generation {
		s.probing = false
	}
}
func (m *Manager) Finish(ctx context.Context, name string, generation uint64, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.entries[name]
	if !ok || generation != s.generation {
		return
	}
	s.probing = false
	if ctx.Err() != nil || errors.Is(err, ErrSuspended) {
		return
	}
	s.LastChecked = m.now()
	missing := err != nil && (strings.Contains(strings.ToLower(err.Error()), "not found") || strings.Contains(strings.ToLower(err.Error()), "status 404") || strings.Contains(strings.ToLower(err.Error()), "http 404"))
	if err == nil || missing {
		s.State = "active"
		s.Reason = ""
		s.ConsecutiveFailures = 0
		s.RetryAt = time.Time{}
		return
	}
	s.ConsecutiveFailures++
	if s.ConsecutiveFailures >= m.threshold {
		s.State = "suspended"
		s.Reason = "consecutive_failures"
		s.RetryAt = m.now().Add(m.cooldown)
		s.generation++
	}
}
func (m *Manager) Snapshot() []Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Status, 0, len(m.entries))
	for _, entry := range m.entries {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Signature changes immediately on registration or circuit transitions, not every failure.
func (m *Manager) Signature() string {
	statuses := m.Snapshot()
	parts := []string{}
	for _, s := range statuses {
		state := s.State
		if state == "suspended" && m.cooldown > 0 && !m.now().Before(s.RetryAt) {
			state = "retry_ready"
			if s.probing {
				state = "probing"
			}
		}
		parts = append(parts, s.Name+":"+state+":"+s.Mode)
	}
	return strings.Join(parts, "|")
}
