package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"planfix-cli/internal/buildinfo"
	"planfix-cli/internal/cmd/auth"
	"planfix-cli/internal/config"
	"planfix-cli/internal/planfix"
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

func init() {
	rootCmd.AddCommand(auth.NewCmd(func() string {
		cfg, err := config.Load(config.ResolvePath())
		if err != nil {
			return "default"
		}
		return config.ResolveProfileName(globalOpts.Profile, cfg)
	}))
	rootCmd.AddCommand(pingCmd)
}

// newClient loads the active profile and builds an API client for it.
func newClient() (*planfix.Client, error) {
	path := config.ResolvePath()
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	name := config.ResolveProfileName(globalOpts.Profile, cfg)
	p, err := config.Resolve(cfg, name)
	if err != nil {
		return nil, err
	}
	if p.Domain == "" || p.Token == "" {
		return nil, fmt.Errorf("profile %q has empty domain or token; run `planfix auth login`", name)
	}
	return planfix.New(p.Domain, p.Token)
}

// Execute runs the root command.
func Execute(ctx context.Context) error {
	rootCmd.SetContext(ctx)
	return rootCmd.Execute()
}

// VersionString returns a human-readable version line.
func VersionString() string {
	return buildinfo.String()
}
