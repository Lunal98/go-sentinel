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
package handlers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"syscall"

	"github.com/Lunal98/go-sentinel/config"

	"github.com/rs/zerolog"
)

// StateHandler defines the interface for state-specific logic.
type StateHandler interface {
	// Start should initiate the state's operation. It receives the state config
	// and a context for cancellation. It should return the *exec.Cmd if it starts a process,
	// or nil if it manages the state internally.
	Start(ctx context.Context, state config.State, log *zerolog.Logger) (*exec.Cmd, error)
	// Stop should gracefully terminate the state's operation.
	Stop(cmd *exec.Cmd, log *zerolog.Logger) error
	// Restart should restart the state's operation, potentially handling parameter changes.
	// It returns the new *exec.Cmd if a process is started, or nil.
	Restart(ctx context.Context, oldCmd *exec.Cmd, state config.State, log *zerolog.Logger) (*exec.Cmd, error)
}
type StateHandlerFactory interface {
	New(state config.State, log *zerolog.Logger) StateHandler
}

// Registry stores registered StateHandlers.
var Registry = make(map[string]StateHandler)
var FactoryRegistry = make(map[string]StateHandlerFactory)

// Register registers a StateHandler for a given type string.
func Register(handlerType string, handler StateHandler) {
	Registry[handlerType] = handler
}
func RegisterType(handlerType string, handlerFactory StateHandlerFactory) {
	FactoryRegistry[handlerType] = handlerFactory
}

// DefaultCmdStateHandler implements StateHandler for "cmd" type states.
type DefaultCmdStateHandler struct{}

// getCmdFromParams extracts the command string from state.Params.
func (h *DefaultCmdStateHandler) getCmdFromParams(state config.State) (string, error) {
	cmdInterface, ok := state.Params["cmd"]
	if !ok {
		return "", fmt.Errorf("missing 'cmd' parameter for state %s", state.Name)
	}
	cmdStr, ok := cmdInterface.(string)
	if !ok {
		return "", fmt.Errorf("'cmd' parameter for state %s is not a string", state.Name)
	}
	return cmdStr, nil
}

// Start implements the Start method for DefaultCmdStateHandler.
func (h *DefaultCmdStateHandler) Start(ctx context.Context, state config.State, log *zerolog.Logger) (*exec.Cmd, error) {
	cmdStr, err := h.getCmdFromParams(state)
	if err != nil {
		return nil, err
	}

	log.Info().Str("name", state.Name).Str("cmd", cmdStr).Msg("Starting default cmd state")

	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return nil, fmt.Errorf("state command is empty for state %s", state.Name)
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start state %s: %w", state.Name, err)
	}
	return cmd, nil
}

// Stop implements the Stop method for DefaultCmdStateHandler.
func (h *DefaultCmdStateHandler) Stop(cmd *exec.Cmd, log *zerolog.Logger) error {
	if cmd != nil && cmd.Process != nil {
		log.Info().Int("pid", cmd.Process.Pid).Msg("Stopping cmd state")
		if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM); err != nil {
			log.Error().Err(err).Msg("Failed to send SIGTERM to process group, attempting to kill main process")
			if err := cmd.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		}
	}
	return nil
}

// Restart implements the Restart method for DefaultCmdStateHandler.
func (h *DefaultCmdStateHandler) Restart(ctx context.Context, oldCmd *exec.Cmd, state config.State, log *zerolog.Logger) (*exec.Cmd, error) {
	if err := h.Stop(oldCmd, log); err != nil {
		log.Error().Err(err).Msg("Error stopping old cmd during restart")
	}
	return h.Start(ctx, state, log)
}
