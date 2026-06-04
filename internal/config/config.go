package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config describes go-prism project configuration.
type Config struct {
	Module string       `yaml:"module"`
	Checks ChecksConfig `yaml:"checks"`
	Policy PolicyConfig `yaml:"policy"`
	AI     AIConfig     `yaml:"ai"`
}

// ChecksConfig controls deterministic checks.
type ChecksConfig struct {
	GoMod      CheckConfig `yaml:"gomod"`
	API        CheckConfig `yaml:"api"`
	Vuln       CheckConfig `yaml:"vuln"`
	Downstream CheckConfig `yaml:"downstream"`
}

// CheckConfig is the common enabled flag for a check family.
type CheckConfig struct {
	Enabled bool `yaml:"enabled"`
}

// PolicyConfig contains early policy toggles.
type PolicyConfig struct {
	FailOn map[string]bool `yaml:"fail_on"`
	WarnOn map[string]bool `yaml:"warn_on"`
}

// AIConfig controls optional AI summary behavior.
type AIConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

// Default returns safe default configuration.
func Default() Config {
	return Config{
		Checks: ChecksConfig{
			GoMod:      CheckConfig{Enabled: true},
			API:        CheckConfig{Enabled: false},
			Vuln:       CheckConfig{Enabled: false},
			Downstream: CheckConfig{Enabled: false},
		},
		Policy: PolicyConfig{
			FailOn: map[string]bool{
				"gomod_parse_error": true,
			},
			WarnOn: map[string]bool{},
		},
		AI: AIConfig{
			Enabled: false,
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
	if cfg.AI.Enabled && cfg.AI.Provider == "" {
		return errors.New("ai.provider is required when ai.enabled is true")
	}
	return nil
}
