package remediation

import (
	"context"

	"github.com/rs/zerolog"
)

type NetworkResetter struct{}

// Start implements Remediator.
func (n *NetworkResetter) Start(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	panic("unimplemented")
}
