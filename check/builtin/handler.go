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

	"github.com/rs/zerolog"
)

// CheckHandler defines the interface for a check that can be executed.
type CheckHandler interface {
	Execute(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error
}

// Registry stores registered CheckHandlers.
var Registry = make(map[string]CheckHandler)

// Register adds a new handler to the global registry.
func Register(actionType string, handler CheckHandler) {
	Registry[actionType] = handler
}
func init() {
	Register("mount", &MountChecker{})
	Register("process", &ProcessChecker{})
}
