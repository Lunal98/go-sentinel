package remediation

import (
	"context"

	"github.com/rs/zerolog"
)

type Remediator interface {
	Start(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error
}

var Registry = make(map[string]Remediator)

// Register adds a new handler to the global registry.
func Register(actionType string, handler Remediator) {
	Registry[actionType] = handler
}
func init() {
	Register("mount", &FSRemounter{})
	Register("process", &StateRestarter{})
}
