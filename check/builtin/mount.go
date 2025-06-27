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
	"fmt"
	"os"
	"strings"

	"github.com/Lunal98/go-sentinel/utils"
	"github.com/rs/zerolog"
)

// MountChecker handles checks that ensure a device is mounted.
type MountChecker struct{}

// Execute performs the mount check and action.
func (h *MountChecker) Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	device, ok := params["device"].(string)
	if !ok || device == "" {
		log.Error().Msg("Mount remediator: 'device' parameter missing or invalid")
		return nil
		//return fmt.Errorf("mount remediator: 'device' parameter missing or invalid")
	}
	dir, ok := params["dir"].(string)
	if !ok || dir == "" {
		log.Error().Msg("Mount remediator: 'dir' parameter missing or invalid")
		return nil
		//return fmt.Errorf("mount remediator: 'dir' parameter missing or invalid")
	}

	log.Debug().Str("device", device).Str("directory", dir).Msg("Checking if mounted")

	mounted, err := utils.IsMounted(device, dir)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check filesystem mount status")
		return nil
		//return fmt.Errorf("failed to check mount status: %w", err)
	}

	if mounted {
		log.Debug().Str("device", device).Str("directory", dir).Msg("Filesystem is already mounted")
		return nil
	}

	log.Info().Str("device", device).Str("directory", dir).Msg("Attempting to mount filesystem")
	fstabContent, err := os.ReadFile("/etc/fstab")
	if err == nil && strings.Contains(string(fstabContent), device+" "+dir) {
		return fmt.Errorf("mount found in /etc/fstab, but not mounted: %s %s", device, dir)
	}
	return nil

}
