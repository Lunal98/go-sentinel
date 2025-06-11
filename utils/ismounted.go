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
package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// IsMounted checks /proc/mounts to see if a specific device is mounted
// at a specific directory.
func IsMounted(device, dir string) (bool, error) {
	// This file contains a list of all currently mounted filesystems.
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("failed to open /proc/mounts: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// Iterate over each line in the /proc/mounts file.
	for scanner.Scan() {
		line := scanner.Text()
		// A typical line format is: "device_name mount_point fstype ..."
		fields := strings.Fields(line)

		// Check if there are at least two fields (device and mount point).
		if len(fields) >= 2 {
			currentDevice := fields[0]
			currentDir := fields[1]

			// Compare the current line's device and directory with the inputs.
			if currentDevice == device && currentDir == dir {
				return true, nil // Found the matching mount entry.
			}
		}
	}

	// Check for any errors encountered during scanning.
	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading /proc/mounts: %w", err)
	}

	// No matching mount entry was found.
	return false, nil
}
