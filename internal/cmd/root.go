package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"planfix-cli/internal/buildinfo"
)

// GlobalOpts holds the persistent flags available to every command.
type GlobalOpts struct {
	JSON    bool
	Fields  string
	Quiet   bool
	Profile string
}

// globalOpts backs the persistent flags; sibling commands in this package read it directly.
var globalOpts GlobalOpts

// NewRootCmd builds the root command with global flags registered.
// Every call returns an isolated command tree, so tests never share state.
func NewRootCmd() *cobra.Command {
	c := &cobra.Command{
		Use:           "planfix",
		Short:         "Command-line client for the Planfix REST API",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
		// Runnable so help/usage renders the global flags when no subcommand matches.
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	c.SetVersionTemplate(VersionString() + "\n")
	pf := c.PersistentFlags()
	pf.BoolVar(&globalOpts.JSON, "json", false, "print raw JSON response")
	pf.StringVar(&globalOpts.Fields, "fields", "", "comma-separated fields to request/show")
	pf.BoolVarP(&globalOpts.Quiet, "quiet", "q", false, "print only the id / drop table header")
	pf.StringVar(&globalOpts.Profile, "profile", "", "config profile name")
	return c
}

// rootCmd is the process-wide tree; subcommands added in this package register onto it.
var rootCmd = NewRootCmd()

// Execute runs the root command.
func Execute(ctx context.Context) error {
	rootCmd.SetContext(ctx)
	return rootCmd.Execute()
}

// VersionString returns a human-readable version line.
func VersionString() string {
	return buildinfo.String()
}
