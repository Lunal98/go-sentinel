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
package state

import (
	"context"
	"encoding/json" // Added for hashing
	"os/exec"
	"sync"
	"syscall"

	"github.com/Lunal98/go-sentinel/config"
	"github.com/Lunal98/go-sentinel/state/handlers" // New import for handlers package

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Manager handles the lifecycle of states (long-running processes).
type Manager struct {
	states        []config.State
	log           *zerolog.Logger
	mu            sync.Mutex
	currentIndex  int
	currentCmd    *exec.Cmd
	managerCtx    context.Context
	managerCancel context.CancelFunc
}

// NewManager creates a new state manager.
func NewManager(states []config.State, logger *zerolog.Logger) *Manager {
	return &Manager{
		states:       states,
		log:          logger,
		currentIndex: -1,
	}
}

// Run starts the initial state and keeps it running until the context is cancelled.
func (m *Manager) Run(ctx context.Context) {
	m.managerCtx, m.managerCancel = context.WithCancel(ctx)

	m.Next()

	<-m.managerCtx.Done()
	m.log.Info().Msg("State manager shutting down. Stopping all commands.")
	m.StopCurrent()
}

// SetStates updates the list of states managed by the Manager.
// If a state is currently running, it will be stopped before the new states are applied.
// The manager will then attempt to start the first state in the new list if available.
// This function is safe for concurrent use.
func (m *Manager) SetStates(newStates []config.State) {
	if statesEqual(m.states, newStates) {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.log.Info().Msg("Updating states in manager.")

	m.stopCurrentUnsafe()

	m.states = newStates
	m.currentIndex = -1

	if len(m.states) > 0 {
		m.log.Info().Msg("New states applied. Starting the first state.")
		m.currentIndex = 0
		m.startCurrentUnsafe()
	} else {
		m.log.Warn().Msg("No states provided in the update. Manager will be idle.")
	}
}

// Next stops the currently running state (if any) and starts the next one in the list.
// This function is safe for concurrent use.
func (m *Manager) Next() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopCurrentUnsafe()

	if len(m.states) == 0 {
		m.log.Warn().Msg("No states configured to run.")
		return
	}

	m.currentIndex = (m.currentIndex + 1) % len(m.states)
	m.startCurrentUnsafe()
}

// CurrentStateName returns the name of the currently active state.
// This function is safe for concurrent use.
func (m *Manager) CurrentStateName() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currentIndex < 0 || m.currentIndex >= len(m.states) {
		return ""
	}
	return m.states[m.currentIndex].Name
}

// startCurrentUnsafe starts the state using its registered handler.
func (m *Manager) startCurrentUnsafe() {
	state := m.states[m.currentIndex]
	m.log.Info().Str("name", state.Name).Str("type", state.Type).Msg("Attempting to start state with handler")

	handler, ok := handlers.HandlerRegistry[state.Type]
	if !ok {
		m.log.Error().Str("state", state.Name).Str("type", state.Type).Msg("No handler registered for this state type")
		return
	}

	cmd, err := handler.Start(m.managerCtx, state, m.log)
	if err != nil {
		m.log.Error().Err(err).Str("state", state.Name).Msg("Failed to start state via handler")
		m.currentCmd = nil
		return
	}

	m.currentCmd = cmd
	if m.currentCmd != nil && m.currentCmd.Process != nil {
		m.log.Info().Str("state", state.Name).Int("pid", m.currentCmd.Process.Pid).Msg("State process started successfully by handler")
		go m.watch(m.currentCmd, state.Name, m.managerCtx)
	} else {
		m.log.Info().Str("state", state.Name).Msg("Handler started state internally or did not return a process")
	}
}

// stopCurrentUnsafe stops the current state using its registered handler.
func (m *Manager) stopCurrentUnsafe() {
	if m.currentCmd == nil && m.currentIndex == -1 {
		return
	}
	if m.currentIndex < 0 || m.currentIndex >= len(m.states) {
		m.log.Debug().Msg("No valid current state to stop.")
		if m.currentCmd != nil && m.currentCmd.Process != nil {
			m.log.Warn().Int("pid", m.currentCmd.Process.Pid).Msg("Force killing orphaned process as no matching state config found.")
			if err := syscall.Kill(-m.currentCmd.Process.Pid, syscall.SIGTERM); err != nil {
				m.log.Error().Err(err).Msg("Failed to send SIGTERM to orphaned process group, attempting to kill main process")
				if err := m.currentCmd.Process.Kill(); err != nil {
					m.log.Error().Err(err).Msg("Failed to kill orphaned process")
				}
			}
		}
		m.currentCmd = nil
		return
	}

	state := m.states[m.currentIndex]
	handler, ok := handlers.HandlerRegistry[state.Type]
	if !ok {
		m.log.Error().Str("state", state.Name).Str("type", state.Type).Msg("No handler registered for this state type, attempting default stop")
		if m.currentCmd != nil && m.currentCmd.Process != nil {
			m.log.Info().Int("pid", m.currentCmd.Process.Pid).Msg("Stopping current state via default SIGTERM")
			if err := syscall.Kill(-m.currentCmd.Process.Pid, syscall.SIGTERM); err != nil {
				m.log.Error().Err(err).Msg("Failed to send SIGTERM to process group, attempting to kill main process")
				if err := m.currentCmd.Process.Kill(); err != nil {
					m.log.Error().Err(err).Msg("Failed to kill process")
				}
			}
		}
		m.currentCmd = nil
		return
	}

	if err := handler.Stop(m.currentCmd, m.log); err != nil {
		m.log.Error().Err(err).Str("state", state.Name).Msg("Failed to stop state via handler")
	}
	m.currentCmd = nil
}

// StopCurrent is a public, thread-safe wrapper around stopCurrentUnsafe for external use.
func (m *Manager) StopCurrent() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopCurrentUnsafe()
}

func (m *Manager) watch(cmd *exec.Cmd, stateName string, ctx context.Context) {
	errChan := make(chan error, 1)
	go func() {
		errChan <- cmd.Wait()
	}()

	select {
	case err := <-errChan:
		m.log.Warn().Err(err).Str("state", stateName).Msg("State process has exited")
	case <-ctx.Done():
		m.log.Info().Str("state", stateName).Msg("Watch goroutine received shutdown signal from manager context.")
		select {
		case <-errChan:
		default:
		}
	}
}

func stateToJSON(s config.State) (string, error) {
	bytes, err := json.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func statesEqual(s1, s2 []config.State) bool {
	if len(s1) != len(s2) {
		return false
	}

	for i := range s1 {
		hash1, err := stateToJSON(s1[i])
		if err != nil {
			log.Warn().Err(err).Msg("states can not be converted to json for comparison, assuming change")
			return false
		}
		hash2, err := stateToJSON(s2[i])
		if err != nil {
			log.Warn().Err(err).Msg("states can not be converted to json for comparison, assuming change")
			return false
		}
		if hash1 != hash2 {
			return false
		}
	}

	return true
}

func (m *Manager) Saferestart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.states) == 0 {
		m.log.Warn().Msg("No states configured to restart.")
		return
	}

	state := m.states[m.currentIndex]
	handler, ok := handlers.HandlerRegistry[state.Type]
	if !ok {
		m.log.Error().Str("state", state.Name).Str("type", state.Type).Msg("No handler registered for this state type, cannot safely restart.")
		m.log.Info().Msg("Performing a default restart of the current state.")
		m.stopCurrentUnsafe()
		m.startCurrentUnsafe()
		return
	}

	m.log.Info().Msg("Performing a safe restart of the current state using its handler.")
	newCmd, err := handler.Restart(m.managerCtx, m.currentCmd, state, m.log)
	if err != nil {
		m.log.Error().Err(err).Str("state", state.Name).Msg("Failed to restart state via handler")
		return
	}
	m.currentCmd = newCmd
	if m.currentCmd != nil && m.currentCmd.Process != nil {
		m.log.Info().Str("state", state.Name).Int("pid", m.currentCmd.Process.Pid).Msg("State process restarted successfully by handler")
		go m.watch(m.currentCmd, state.Name, m.managerCtx)
	} else {
		m.log.Info().Str("state", state.Name).Msg("Handler restarted state internally or did not return a new process")
	}
}
