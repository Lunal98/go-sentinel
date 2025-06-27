package remediation

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/rs/zerolog"
)

// FSRemounter handles remediating mount checks by ensuring a device is mounted.
type FSRemounter struct{}

func (h *FSRemounter) Start(ctx context.Context, log *zerolog.Logger, params map[string]interface{}) error {
	log.Debug().Msg("Executing mount -a")
	cmd := exec.CommandContext(ctx, "sudo", "mount", "-a")
	_, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute mount -a: %w", err)
	}
	return nil
}
