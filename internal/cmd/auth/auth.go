// Package auth implements the planfix auth login/status/logout subcommands.
package auth

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/6insanes/planfix-cli/internal/config"
	"github.com/6insanes/planfix-cli/internal/planfix"
)

// openClient builds an API client; tests swap it to point at a fake server.
var openClient = planfix.New

// NewCmd builds the auth command tree. resolveName reports the active
// profile name — the parent package owns the global flags, so the name
// is injected instead of read directly.
func NewCmd(resolveName func() string) *cobra.Command {
	c := &cobra.Command{
		Use:   "auth",
		Short: "Manage credentials and profiles",
	}
	c.AddCommand(newLoginCmd(resolveName))
	c.AddCommand(newStatusCmd(resolveName))
	c.AddCommand(newLogoutCmd(resolveName))
	return c
}

func newLoginCmd(resolveName func() string) *cobra.Command {
	var domain, token, name string
	var force bool
	c := &cobra.Command{
		Use:   "login",
		Short: "Store credentials as a profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ResolvePath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			if name == "" {
				name = resolveName()
			}
			if _, exists := cfg.Profiles[name]; exists && !force {
				return fmt.Errorf("profile %q already exists; pass --force to overwrite", name)
			}
			domain, token, err = promptCredentials(cmd, domain, token)
			if err != nil {
				return err
			}
			p := &config.Profile{Domain: domain, Token: token}
			if old := cfg.Profiles[name]; old != nil {
				p.Worklog = old.Worklog
			}
			cfg.Profiles[name] = p
			cfg.CurrentProfile = name
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Logged in: profile %q, domain %s, token %s\n", name, domain, mask(token))
			return nil
		},
	}
	f := c.Flags()
	f.StringVar(&domain, "domain", "", "account domain, e.g. example.planfix.ru")
	f.StringVar(&token, "token", "", "API token (prompted for if empty)")
	f.StringVar(&name, "name", "", "profile name (default: active profile)")
	f.BoolVar(&force, "force", false, "overwrite an existing profile")
	return c
}

func newStatusCmd(resolveName func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the active profile and check API access",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.ResolvePath())
			if err != nil {
				return err
			}
			name := resolveName()
			p, err := config.Resolve(cfg, name)
			if err != nil {
				return err
			}
			if p.Domain == "" || p.Token == "" {
				return fmt.Errorf("profile %q has empty domain or token; run `planfix auth login`", name)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "profile: %s\n", name)
			fmt.Fprintf(out, "domain:  %s\n", p.Domain)
			fmt.Fprintf(out, "token:   %s\n", mask(p.Token))
			c, err := openClient(p.Domain, p.Token)
			if err != nil {
				return err
			}
			if _, err := c.JSON(cmd.Context(), http.MethodGet, "/ping", nil); err != nil {
				return planfix.WrapHint(err)
			}
			fmt.Fprintln(out, "ping:    OK")
			return nil
		},
	}
}

func newLogoutCmd(resolveName func() string) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove a stored profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := config.ResolvePath()
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			name := resolveName()
			if _, ok := cfg.Profiles[name]; !ok {
				return fmt.Errorf("profile %q not found", name)
			}
			delete(cfg.Profiles, name)
			if cfg.CurrentProfile == name {
				cfg.CurrentProfile = ""
			}
			if err := config.Save(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed profile %q\n", name)
			return nil
		},
	}
}

// promptCredentials fills in missing domain/token via stdin prompts,
// written to stderr so stdout stays machine-readable.
// The token is hidden when stdin is a terminal.
func promptCredentials(cmd *cobra.Command, domain, token string) (string, string, error) {
	prompt := cmd.ErrOrStderr()
	rdr := bufio.NewReader(cmd.InOrStdin())
	if domain == "" {
		fmt.Fprint(prompt, "Domain (e.g. example.planfix.ru): ")
		line, err := readLine(rdr)
		if err != nil {
			return "", "", fmt.Errorf("read domain: %w", err)
		}
		domain = strings.TrimSpace(line)
		if domain == "" {
			return "", "", fmt.Errorf("domain must not be empty")
		}
	}
	if token == "" {
		fmt.Fprint(prompt, "Token: ")
		if f, ok := cmd.InOrStdin().(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			b, err := term.ReadPassword(int(f.Fd()))
			fmt.Fprintln(prompt)
			if err != nil {
				return "", "", fmt.Errorf("read token: %w", err)
			}
			token = strings.TrimSpace(string(b))
		} else {
			line, err := readLine(rdr)
			if err != nil {
				return "", "", fmt.Errorf("read token: %w", err)
			}
			token = strings.TrimSpace(line)
		}
		if token == "" {
			return "", "", fmt.Errorf("token must not be empty")
		}
	}
	return domain, token, nil
}

// readLine reads one line, treating a trailing EOF without a newline as success.
func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return line, nil
}

// mask hides all but the first and last two runes of a secret.
// Secrets shorter than eight bytes (or fewer than five runes) are fully hidden.
func mask(s string) string {
	if len(s) < 8 {
		return "****"
	}
	r := []rune(s)
	if len(r) < 5 {
		return "****"
	}
	return string(r[:2]) + "****" + string(r[len(r)-2:])
}
