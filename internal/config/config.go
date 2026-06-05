package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config describes go-prism project configuration.
type Config struct {
	Module string       `yaml:"module"`
	Checks ChecksConfig `yaml:"checks"`
	Policy PolicyConfig `yaml:"policy"`
}

// ChecksConfig controls deterministic checks.
type ChecksConfig struct {
	GoMod      CheckConfig      `yaml:"gomod"`
	API        CheckConfig      `yaml:"api"`
	Vuln       CheckConfig      `yaml:"vuln"`
	Downstream DownstreamConfig `yaml:"downstream"`
}

// CheckConfig is the common enabled flag for a check family.
type CheckConfig struct {
	Enabled bool `yaml:"enabled"`
}

// DownstreamConfig controls downstream canary checks.
type DownstreamConfig struct {
	Enabled bool                     `yaml:"enabled"`
	Modules []DownstreamModuleConfig `yaml:"modules"`
}

// DownstreamModuleConfig describes one downstream consumer module.
type DownstreamModuleConfig struct {
	Name    string `yaml:"name"`
	Path    string `yaml:"path"`
	Repo    string `yaml:"repo"`
	Ref     string `yaml:"ref"`
	Subdir  string `yaml:"subdir"`
	Command string `yaml:"command"`
}

// PolicyConfig contains early policy toggles.
type PolicyConfig struct {
	FailOn map[string]bool `yaml:"fail_on"`
	WarnOn map[string]bool `yaml:"warn_on"`
}

// Default returns safe default configuration.
func Default() Config {
	return Config{
		Checks: ChecksConfig{
			GoMod:      CheckConfig{Enabled: true},
			API:        CheckConfig{Enabled: false},
			Vuln:       CheckConfig{Enabled: false},
			Downstream: DownstreamConfig{Enabled: false},
		},
		Policy: PolicyConfig{
			FailOn: map[string]bool{
				"gomod_parse_error": true,
			},
			WarnOn: map[string]bool{},
		},
	}
}

// Load reads a YAML config. If the default config file is missing, defaults are used.
func Load(path string) (Config, error) {
	cfg := Default()
	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && path == ".go-prism.yml" {
			return cfg, nil
		}
		return Config{}, fmt.Errorf("load config %q: %w", path, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	applyDefaults(&cfg)
	return cfg, validate(cfg)
}

func applyDefaults(cfg *Config) {
	if cfg.Policy.FailOn == nil {
		cfg.Policy.FailOn = map[string]bool{}
	}
	if cfg.Policy.WarnOn == nil {
		cfg.Policy.WarnOn = map[string]bool{}
	}
	if !cfg.Checks.GoMod.Enabled &&
		!cfg.Checks.API.Enabled &&
		!cfg.Checks.Vuln.Enabled &&
		!cfg.Checks.Downstream.Enabled {
		cfg.Checks.GoMod.Enabled = true
	}
}

func validate(cfg Config) error {
	for i, module := range cfg.Checks.Downstream.Modules {
		if strings.TrimSpace(module.Name) == "" {
			return fmt.Errorf("checks.downstream.modules[%d].name is required", i)
		}
		path := strings.TrimSpace(module.Path)
		repo := strings.TrimSpace(module.Repo)
		switch {
		case path == "" && repo == "":
			return fmt.Errorf("checks.downstream.modules[%d].path or repo is required", i)
		case path != "" && repo != "":
			return fmt.Errorf("checks.downstream.modules[%d].path and repo are mutually exclusive", i)
		case repo != "":
			if err := validateRemoteRepo(repo); err != nil {
				return fmt.Errorf("checks.downstream.modules[%d].repo: %w", i, err)
			}
			if err := validateRemoteSubdir(module.Subdir); err != nil {
				return fmt.Errorf("checks.downstream.modules[%d].subdir: %w", i, err)
			}
		}
	}
	return nil
}

func validateRemoteRepo(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return err
	}
	if parsed.Scheme != "https" {
		return errors.New("remote downstream repo must use https")
	}
	if parsed.Host == "" {
		return errors.New("remote downstream repo host is required")
	}
	if parsed.User != nil {
		return errors.New("remote downstream repo must not include credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("remote downstream repo must not include query or fragment")
	}
	return nil
}

func validateRemoteSubdir(raw string) error {
	subdir := strings.TrimSpace(raw)
	if subdir == "" || subdir == "." {
		return nil
	}
	if filepath.IsAbs(subdir) {
		return errors.New("remote downstream subdir must be relative")
	}
	clean := filepath.Clean(subdir)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("remote downstream subdir must not escape the clone")
	}
	return nil
}
