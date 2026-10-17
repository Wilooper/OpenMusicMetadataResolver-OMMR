package bootstrap

import (
	"context"
	"sync"

	"github.com/ommr/ommr/internal/adapters"
	"github.com/ommr/ommr/internal/providerhealth"
)

type Candidate struct {
	Source adapters.ProviderAdapter
	Mode   string
	Check  func(context.Context) error
}
type Plan struct {
	Name       string
	Enabled    bool
	Configured bool
	Candidates []Candidate
	Public     adapters.ProviderAdapter
}

// Activate validates supplied modes before registration. Public fallback is explicit.
func Activate(ctx context.Context, plans []Plan, workers int, manager *providerhealth.Manager, registry *adapters.Registry) {
	for _, plan := range plans {
		registry.MarkKnown(plan.Name)
	}
	if workers < 1 {
		workers = 4
	}
	jobs := make(chan Plan)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for plan := range jobs {
				if !plan.Enabled {
					manager.Set(plan.Name, "disabled", "off", "unused", "disabled_by_config")
					continue
				}
				loaded := false
				for _, candidate := range plan.Candidates {
					if candidate.Check != nil && candidate.Check(ctx) == nil {
						manager.Set(plan.Name, "active", candidate.Mode, "validated", "")
						registry.Register(providerhealth.Wrap(candidate.Source, manager))
						loaded = true
						break
					}
				}
				if loaded {
					continue
				}
				if plan.Public != nil {
					credentialState := "absent"
					reason := ""
					if plan.Configured {
						credentialState = "rejected"
						reason = "credential_probe_failed"
					}
					manager.Set(plan.Name, "active", "public", credentialState, reason)
					registry.Register(providerhealth.Wrap(plan.Public, manager))
					continue
				}
				reason := "credentials_missing"
				state := "absent"
				if plan.Configured {
					reason = "credential_probe_failed"
					state = "rejected"
				}
				manager.Set(plan.Name, "disabled", "off", state, reason)
			}
		}()
	}
	for _, plan := range plans {
		jobs <- plan
	}
	close(jobs)
	wg.Wait()
	registry.SetAvailability(manager.Available)
}
