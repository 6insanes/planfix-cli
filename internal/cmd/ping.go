package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/6insanes/planfix-cli/internal/planfix"
)

// newPingCmd builds the ping subcommand.
func newPingCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ping",
		Short: "Check credentials with GET /ping",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newClient()
			if err != nil {
				return err
			}
			return doPing(cmd.Context(), c, cmd.OutOrStdout())
		},
	}
}

// doPing calls GET /ping and prints OK. Known Planfix app codes are
// wrapped with their actionable hint.
func doPing(ctx context.Context, c *planfix.Client, out io.Writer) error {
	if _, err := c.JSON(ctx, http.MethodGet, "/ping", nil); err != nil {
		return planfix.WrapHint(err)
	}
	_, _ = fmt.Fprintln(out, "OK")
	return nil
}
