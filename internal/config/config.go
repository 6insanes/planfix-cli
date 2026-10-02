// Package config loads and saves planfix CLI profiles.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WorklogMeta caches the account's worklog data-tag field ids.
type WorklogMeta struct {
	DataTagID         int `yaml:"datatag_id"`
	FieldDate         int `yaml:"field_date"`
	FieldTime         int `yaml:"field_time"`
	FieldWorkType     int `yaml:"field_work_type,omitempty"`
	FieldEmployee     int `yaml:"field_employee,omitempty"`
	WorkTypeDirectory int `yaml:"work_type_directory,omitempty"`
}

// Profile is one named account.
type Profile struct {
	Domain  string       `yaml:"domain"`
	Token   string       `yaml:"token"`
	Worklog *WorklogMeta `yaml:"worklog,omitempty"`
}

// Config is the on-disk profile file.
type Config struct {
	CurrentProfile string              `yaml:"current_profile"`
	Profiles       map[string]*Profile `yaml:"profiles"`
}

// ResolvePath returns the config file path (PLANFIX_CONFIG or default).
func ResolvePath() string {
	if p := os.Getenv("PLANFIX_CONFIG"); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yml"
	}
	return filepath.Join(home, ".config", "planfix", "config.yml")
}

// ResolveProfileName picks the active profile name.
// Precedence: flag > PLANFIX_PROFILE > current_profile > "default".
func ResolveProfileName(flag string, c *Config) string {
	if flag != "" {
		return flag
	}
	if env := os.Getenv("PLANFIX_PROFILE"); env != "" {
		return env
	}
	if c != nil && c.CurrentProfile != "" {
		return c.CurrentProfile
	}
	return "default"
}

// Load reads the config file. A missing file yields an empty config.
func Load(path string) (*Config, error) {
	c := &Config{Profiles: map[string]*Profile{}}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*Profile{}
	}
	return c, nil
}

// Save writes the config atomically with mode 0600.
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Resolve returns the profile with env overrides applied
// (PLANFIX_DOMAIN, PLANFIX_TOKEN override file values).
func Resolve(c *Config, name string) (*Profile, error) {
	p, ok := c.Profiles[name]
	if !ok || p == nil {
		return nil, fmt.Errorf("profile %q not found; run `planfix auth login`", name)
	}
	out := *p
	if d := os.Getenv("PLANFIX_DOMAIN"); d != "" {
		out.Domain = d
	}
	if t := os.Getenv("PLANFIX_TOKEN"); t != "" {
		out.Token = t
	}
	return &out, nil
}
