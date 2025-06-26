/*
Copyright © 2025 Alex Bedo <alex98hun@gmail.com>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package check

import (
	"context"
	"sync"
	"time"

	"github.com/Lunal98/go-sentinel/check/builtin"
	"github.com/Lunal98/go-sentinel/check/remediation"
	"github.com/Lunal98/go-sentinel/config"
	"github.com/Lunal98/go-sentinel/state"

	"github.com/rs/zerolog"
)

// Scheduler manages the execution of one-shot and periodic Checks.
type Scheduler struct {
	Checks      []config.Check
	log         *zerolog.Logger
	mu          sync.Mutex
	cancelFuncs map[string]context.CancelFunc
}

type CheckHandler = builtin.CheckHandler
type Remediator = remediation.Remediator

// NewScheduler creates a new Check scheduler.
func NewScheduler(Checks []config.Check, logger *zerolog.Logger) *Scheduler {
	return &Scheduler{
		Checks:      Checks,
		log:         logger,
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

func (s *Scheduler) RegisterHandler(name string, handl builtin.CheckHandler) {
	builtin.Register(name, handl)
}
func (s *Scheduler) RegisterRemediator(name string, remediator remediation.Remediator) {
	remediation.Register(name, remediator)
}

// Run starts the scheduler, which executes Checks based on their frequency.
func (s *Scheduler) Run(ctx context.Context, sm *state.Manager) {
	s.log.Info().Msg("Check scheduler started")

	s.mu.Lock()
	s.initializeChecks(ctx, sm)
	s.mu.Unlock()

	<-ctx.Done()
	s.log.Info().Msg("Check scheduler shutting down.")
	s.mu.Lock()
	s.stopAllPeriodicChecksUnsafe()
	s.mu.Unlock()
}

func (s *Scheduler) initializeChecks(ctx context.Context, sm *state.Manager) {
	s.stopAllPeriodicChecksUnsafe()

	for _, Check := range s.Checks {
		switch Check.Frequency.Type {
		case "oneshot":
			go s.executeCheck(ctx, Check, sm)
		case "periodic":
			CheckCtx, cancel := context.WithCancel(ctx)
			s.cancelFuncs[Check.Name] = cancel
			go s.runPeriodicChecks(CheckCtx, Check, sm)
		default:
			s.log.Error().Str("Check", Check.Name).Str("type", Check.Frequency.Type).Msg("Unknown Check frequency type")
		}
	}
}

// SetChecks updates the list of Checks managed by the Scheduler.
// This function is safe for concurrent use. Any previously scheduled periodic Checks will be stopped,
// and the new Checks will be scheduled.
func (s *Scheduler) SetChecks(newChecks []config.Check, sm *state.Manager) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info().Msg("Updating Checks in scheduler.")

	s.stopAllPeriodicChecksUnsafe()

	s.Checks = newChecks

	s.initializeChecks(context.Background(), sm)
}

func (s *Scheduler) stopAllPeriodicChecksUnsafe() {
	for CheckName, cancel := range s.cancelFuncs {
		s.log.Info().Str("Check", CheckName).Msg("Stopping periodic Check.")
		cancel()
		delete(s.cancelFuncs, CheckName)
	}
}

func (s *Scheduler) runPeriodicChecks(ctx context.Context, Check config.Check, sm *state.Manager) {
	duration, err := time.ParseDuration(Check.Frequency.Time)
	if err != nil {
		s.log.Error().Err(err).Str("Check", Check.Name).Msg("Invalid time duration for periodic Check")
		return
	}

	s.log.Info().Str("Check", Check.Name).Str("interval", duration.String()).Msg("Scheduling periodic Check")
	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.executeCheck(ctx, Check, sm)
		case <-ctx.Done():
			s.log.Debug().Str("Check", Check.Name).Msg("Stopping periodic Check due to context cancellation.")
			return
		}
	}
}

func (s *Scheduler) executeCheck(ctx context.Context, Check config.Check, sm *state.Manager) {
	if Check.Condition != nil && Check.Condition.State != "" {
		if sm.CurrentStateName() != Check.Condition.State {
			s.log.Debug().Str("Check", Check.Name).
				Str("required_state", Check.Condition.State).
				Str("active_state", sm.CurrentStateName()).
				Msg("Skipping Check due to state condition not met")
			return
		}
	}

	handler, ok := builtin.Registry[Check.Action.Type]
	if !ok {
		s.log.Error().Str("Check", Check.Name).Str("type", Check.Action.Type).Msg("No handler registered for action type")
		return
	}

	s.log.Debug().Str("Check", Check.Name).Msg("Executing Check")
	err := handler.Execute(ctx, s.log, Check.Action.Params)
	if err == nil {
		s.log.Debug().Str("Check", Check.Name).Msg("Check executed successfully")
	} else if err == builtin.ErrProcessNotRunning {
		s.log.Warn().Msg("Check executed successfully, but process is not running, restarting state")
		sm.Saferestart()
	} else {
		s.log.Error().Err(err).Str("Check", Check.Name).Msg("Check failed, starting remediation")
		s.handleRemediation(ctx, Check, sm)
	}
}

func (s *Scheduler) handleRemediation(ctx context.Context, Check config.Check, sm *state.Manager) {
	if len(Check.Remediation) == 0 {
		s.log.Warn().Str("Check", Check.Name).Msg("No remediation actions defined for this Check")
		return
	}

	for _, rem := range Check.Remediation {
		remediator, ok := remediation.Registry[rem.Type]
		if !ok {
			s.log.Error().Str("check", Check.Name).Str("remediation", rem.Type).Msg("No remediator registered for this type")
			continue
		}

		if rem.Before != "" {
			delay, err := time.ParseDuration(rem.Before)
			if err != nil {
				s.log.Error().Err(err).Str("Check", Check.Name).Msg("Invalid delay duration for remediation")
				continue
			}
			s.log.Info().Str("check", Check.Name).Str("remediation", rem.Type).Dur("delay", delay).Msg("Delaying remediation")
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				s.log.Debug().Str("Check", Check.Name).Msg("Stopping remediation due to context cancellation.")
				return
			}
		}

		if err := remediator.Start(ctx, s.log, rem.Params); err != nil {
			s.log.Error().Err(err).Str("check", Check.Name).Str("remediation", rem.Type).Msg("Failed to start remediation")
			continue
		}

		if rem.After != "" {
			delay, err := time.ParseDuration(rem.After)
			if err != nil {
				s.log.Error().Err(err).Str("Check", Check.Name).Msg("Invalid post-remediation delay duration")
				continue
			}
			s.log.Info().Str("check", Check.Name).Str("remediation", rem.Type).Dur("delay", delay).Msg("Delaying post-remediation check")
			select {
			case <-time.After(delay):
				handler, ok := builtin.Registry[Check.Action.Type]
				if !ok {
					s.log.Error().Str("Check", Check.Name).Str("type", Check.Action.Type).Msg("Handler not found for post-remediation check")
					return
				}
				s.log.Debug().Str("check", Check.Name).Str("remediation", rem.Type).Msg("Executing post-remediation check")
				err := handler.Execute(ctx, s.log, Check.Action.Params)
				if err != nil {
					s.log.Debug().Msg("Problem not resolved after remediation")
				} else {
					s.log.Info().Str("check", Check.Name).Str("remediation", rem.Type).Msg("Remediation successful, post-remediation check passed")
				}

			case <-ctx.Done():
				s.log.Debug().Str("Check", Check.Name).Msg("Stopping post-remediation check due to context cancellation.")
				return
			}
		}
	}
	s.log.Error().Str("Check", Check.Name).Msg("Remediation actions completed without resolving the issue")
}
