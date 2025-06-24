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
package builtin

import (
	"context"
	"errors"
	"fmt"

	"github.com/Lunal98/go-sentinel/utils"
	"github.com/rs/zerolog"
)

// ProcessChecker handles checks that check for a running process.
type ProcessChecker struct{}

var ErrProcessNotRunning = errors.New("process not running")

// Execute checks if a process with a given name is running.
func (h *ProcessChecker) Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	procName, ok := params["procname"].(string)
	if !ok || procName == "" {
		return fmt.Errorf("process check: 'procname' parameter missing or invalid")
	}

	found, err := utils.IsProcessRunning(procName)
	if err != nil {
		return fmt.Errorf("error checking for process '%s': %w", procName, err)
	}

	if !found {
		// This is a check, so we log a warning rather than returning an error,
		// as a non-running process may not be a fatal error for the daemon itself.
		return ErrProcessNotRunning
	} else {
		log.Debug().Str("process_name", procName).Msg("Checked for process, and it is running")
	}

	return nil
}
