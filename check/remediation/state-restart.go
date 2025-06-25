package remediation

import (
	"context"

	"github.com/rs/zerolog"
)

type StateRestarter struct{}

// Start implements Remediator.
func (s *StateRestarter) Start(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	panic("unimplemented")
}
