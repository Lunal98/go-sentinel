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
package task

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

// ProcessTaskHandler handles tasks that check for a running process.
type ProcessTaskHandler struct{}

var ErrProcessNotRunning = errors.New("checked for process, but it was not found")

// Execute checks if a process with a given name is running.
func (h *ProcessTaskHandler) Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	procName, ok := params["procname"].(string)
	if !ok || procName == "" {
		return fmt.Errorf("process task: 'procname' parameter missing or invalid")
	}

	found, err := isProcessRunning(procName)
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

// isProcessRunning iterates through the /proc filesystem to find a running process.
// It checks if the command line in /proc/[pid]/cmdline contains the target name.
func isProcessRunning(name string) (bool, error) {
	// Read all entries in the /proc directory.
	dirs, err := os.ReadDir("/proc")
	if err != nil {
		return false, fmt.Errorf("failed to read /proc: %w", err)
	}

	for _, dir := range dirs {
		// We only care about directories that are PIDs (i.e., numbers).
		if !dir.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join("/proc", dir.Name(), "cmdline")); err != nil {
			continue // Not a process directory or we can't access it.
		}

		// Read the command line for the process.
		cmdlinePath := filepath.Join("/proc", dir.Name(), "cmdline")
		cmdlineBytes, err := os.ReadFile(cmdlinePath)
		if err != nil {
			continue // Process might have terminated, or permissions issue.
		}

		// The cmdline file uses null bytes to separate arguments.
		cmdline := string(cmdlineBytes)
		if strings.Contains(cmdline, name) {
			return true, nil // Found the process.
		}
	}

	return false, nil // Did not find the process.
}
