package remediation

import (
	"context"
	"fmt"

	"github.com/Lunal98/go-sentinel/state"
	"github.com/rs/zerolog"
)

type StateRestarter struct {
	Sm *state.Manager
}

// Start implements Remediator.
func (s *StateRestarter) Start(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	if s.Sm == nil {
		return fmt.Errorf("state manager is not initialized")
	}
	s.Sm.Saferestart()
	return nil
}
