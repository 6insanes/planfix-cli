package cmd

import (
	"context"
	"fmt"

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

var globalOpts GlobalOpts

var rootCmd = &cobra.Command{
	Use:           "planfix",
	Short:         "Command-line client for the Planfix REST API",
	SilenceUsage:  true,
	SilenceErrors: true,
	Version:       buildinfo.Version,
	// Runnable so help/usage renders the global flags when no subcommand matches.
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

func init() {
	rootCmd.SetVersionTemplate(VersionString() + "\n")
	pf := rootCmd.PersistentFlags()
	pf.BoolVar(&globalOpts.JSON, "json", false, "print raw JSON response")
	pf.StringVar(&globalOpts.Fields, "fields", "", "comma-separated fields to request/show")
	pf.BoolVarP(&globalOpts.Quiet, "quiet", "q", false, "print only the id / drop table header")
	pf.StringVar(&globalOpts.Profile, "profile", "", "config profile name")
}

// Execute runs the root command.
func Execute(ctx context.Context) error {
	rootCmd.SetContext(ctx)
	return rootCmd.Execute()
}

// GlobalOptions returns the parsed global flags.
func GlobalOptions() GlobalOpts { return globalOpts }

// VersionString returns a human-readable version line.
func VersionString() string {
	return fmt.Sprintf("planfix %s (commit %s, built %s)", buildinfo.Version, buildinfo.Commit, buildinfo.Date)
}
