package t1k

import (
	"fmt"
	"os"

	"github.com/enerplanet/T1K/config"
	"github.com/enerplanet/T1K/internal/mapping"
)

// Config is a parsed and validated mapping configuration. It is immutable
// after loading and safe to share between any number of TransformTasks.
type Config struct {
	// Name identifies the mapping, for example "enerplanet-to-meme".
	Name string
	// Version is the mapping's own version string, as written by its author.
	Version string
	// Description explains what the forward transformation produces.
	Description string

	program *mapping.Program
}

// LoadConfig parses a mapping configuration from its JSON text. Errors wrap
// ErrConfig.
func LoadConfig(data []byte) (*Config, error) {
	program, err := mapping.Compile(data)
	if err != nil {
		return nil, err
	}
	return &Config{Name: program.Name, Version: program.Version, Description: program.Description, program: program}, nil
}

// LoadConfigFile reads and parses a mapping configuration file. Errors wrap
// ErrConfig and name the file.
func LoadConfigFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConfig, err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Rules returns the number of top-level rules, for diagnostics.
func (c *Config) Rules() int { return c.program.RuleCount() }

// defaultConfigJSON is the mapping shipped with the package: the conversion
// of an EnerPlanET calculation payload into a MEME job, embedded from the
// repository's config directory by the config package.
var defaultConfigJSON = mustReadDefault()

// defaultConfig is parsed once, when the package initialises, and shared by
// every task created without an explicit configuration.
var defaultConfig = mustLoadConfig(defaultConfigJSON)

func mustReadDefault() []byte {
	data, err := config.Read(config.Default)
	if err != nil {
		panic(fmt.Sprintf("t1k: embedded default configuration %s is missing: %v", config.Default, err))
	}
	return data
}

func mustLoadConfig(data []byte) *Config {
	cfg, err := LoadConfig(data)
	if err != nil {
		panic(fmt.Sprintf("t1k: embedded default configuration is invalid: %v", err))
	}
	return cfg
}

// DefaultConfig returns the mapping configuration embedded from
// config/enerplanet-to-meme.json in the repository.
func DefaultConfig() *Config { return defaultConfig }

// DefaultConfigJSON returns a copy of the embedded default configuration's
// JSON text, for example to write it out as a starting point for a custom
// mapping.
func DefaultConfigJSON() []byte { return append([]byte(nil), defaultConfigJSON...) }
