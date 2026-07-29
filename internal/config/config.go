// Package config loads gwtui's user configuration.
//
// The canonical source is an XDG-resolved config.toml:
//
//	$XDG_CONFIG_HOME/gwtui/config.toml   if $XDG_CONFIG_HOME is set and non-empty
//	~/.config/gwtui/config.toml          otherwise
//
// Missing or malformed files are non-fatal: defaults are returned and a single
// warning is printed to stderr.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the parsed gwtui configuration.
type Config struct {
	// ShowRepos controls whether main repo checkouts are displayed in orgroot
	// mode. It is only meaningful when ShowReposSet is true; the default (false)
	// is applied at the call site so that an absent key is distinguishable from
	// an explicit false.
	ShowRepos    bool
	ShowReposSet bool
}

// fileConfig is the on-disk decode target for config.toml. Pointer fields
// distinguish absent keys from explicit zero values.
type fileConfig struct {
	ShowRepos *bool `toml:"show_repos,omitempty"`
}

// Load reads and parses config.toml at the XDG-resolved path.
//
// Missing config.toml: returns an empty config (defaults).
// Malformed config.toml: prints one warning to stderr and returns an empty
// config. This is not a fatal error.
//
// The error is reserved for cases where the config path itself cannot be
// resolved (e.g. home directory unknown); callers treat it as non-fatal.
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, nil
		}
		fmt.Fprintf(os.Stderr, "gwtui: warning: could not read %s: %v; using defaults\n", path, err)
		return Config{}, nil
	}

	var fc fileConfig
	if _, err := toml.Decode(string(data), &fc); err != nil {
		fmt.Fprintf(os.Stderr, "gwtui: warning: could not parse %s: %v; using defaults\n", path, err)
		return Config{}, nil
	}

	return fc.normalize(), nil
}

// configPath resolves the config.toml location, honoring $XDG_CONFIG_HOME.
func configPath() (string, error) {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "gwtui", "config.toml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "gwtui", "config.toml"), nil
}

// normalize converts a decoded fileConfig into a Config.
func (fc fileConfig) normalize() Config {
	if fc.ShowRepos != nil {
		return Config{ShowRepos: *fc.ShowRepos, ShowReposSet: true}
	}
	return Config{}
}
