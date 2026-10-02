package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"planfix-cli/internal/buildinfo"
	"planfix-cli/internal/cmd/auth"
	"planfix-cli/internal/cmd/comment"
	"planfix-cli/internal/cmd/task"
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

// NewRootCmd builds the root command with global flags and the
// auth/ping/task subcommands registered. Every call returns an isolated
// command tree, so tests never share state.
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
	c.AddCommand(auth.NewCmd(func() string {
		cfg, err := config.Load(config.ResolvePath())
		if err != nil {
			return "default"
		}
		return config.ResolveProfileName(globalOpts.Profile, cfg)
	}))
	c.AddCommand(newPingCmd())
	c.AddCommand(task.NewCmd(newClient, func() task.Options {
		return task.Options{
			JSON:   globalOpts.JSON,
			Fields: globalOpts.Fields,
			Quiet:  globalOpts.Quiet,
		}
	}, activeDomain))
	c.AddCommand(comment.NewCmd(newClient, func() comment.Options {
		return comment.Options{
			JSON:   globalOpts.JSON,
			Fields: globalOpts.Fields,
			Quiet:  globalOpts.Quiet,
		}
	}))
	return c
}

// rootCmd is the process-wide tree used by Execute.
var rootCmd = NewRootCmd()

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

// activeDomain resolves the active profile's domain for URL printing,
// falling back to the default Planfix domain when no profile is usable.
func activeDomain() string {
	cfg, err := config.Load(config.ResolvePath())
	if err != nil {
		return "example.planfix.ru"
	}
	name := config.ResolveProfileName(globalOpts.Profile, cfg)
	p, err := config.Resolve(cfg, name)
	if err != nil || p.Domain == "" {
		return "example.planfix.ru"
	}
	return planfix.NormalizeDomain(p.Domain)
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
