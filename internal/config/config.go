package config

import (
	"flag"
	"os"
	"strings"
)

const trueValue = "true"

// OperatorConfig holds the configuration for the operator
type OperatorConfig struct {
	// NamespaceRegex is a list of regular expressions to match namespaces
	NamespaceRegex []string

	// DryRun indicates whether to perform actual changes or just log what would be done
	DryRun bool

	// Debug enables debug logging
	Debug bool

	// Trace enables trace logging (more verbose than debug)
	Trace bool

	// Internal field to store the namespace regex string for later parsing
	namespaceRegexStr *string
}

// NewConfig creates a new configuration from command line flags and environment variables
func NewConfig() *OperatorConfig {
	config := &OperatorConfig{}

	var namespaceRegexStr string
	flag.StringVar(&namespaceRegexStr, "namespace-regex", "",
		"Comma-separated list of regex patterns to match namespaces (default: all namespaces)")
	flag.BoolVar(&config.DryRun, "dry-run", false,
		"If true, only log what changes would be made without actually making them")
	flag.BoolVar(&config.Debug, "debug", false,
		"Enable debug logging")
	flag.BoolVar(&config.Trace, "trace", false,
		"Enable trace logging (implies debug)")

	// Store the namespace regex string reference for later parsing
	config.namespaceRegexStr = &namespaceRegexStr

	return config
}

// FinalizeConfig should be called after flag.Parse() to complete configuration
func (c *OperatorConfig) FinalizeConfig() {
	// Parse namespace regex patterns from flags
	if c.namespaceRegexStr != nil && *c.namespaceRegexStr != "" {
		c.NamespaceRegex = strings.Split(*c.namespaceRegexStr, ",")
		for i, pattern := range c.NamespaceRegex {
			c.NamespaceRegex[i] = strings.TrimSpace(pattern)
		}
	}

	// Override with environment variables if present
	if envRegex := os.Getenv("NAMESPACE_REGEX"); envRegex != "" {
		c.NamespaceRegex = strings.Split(envRegex, ",")
		for i, pattern := range c.NamespaceRegex {
			c.NamespaceRegex[i] = strings.TrimSpace(pattern)
		}
	}

	if os.Getenv("DRY_RUN") == trueValue {
		c.DryRun = true
	}

	if os.Getenv("DEBUG") == trueValue {
		c.Debug = true
	}

	if os.Getenv("TRACE") == trueValue {
		c.Trace = true
		c.Debug = true // Trace implies debug
	}
}

// LogLevel returns the appropriate log level based on configuration
func (c *OperatorConfig) LogLevel() int {
	if c.Trace {
		return -2 // Very verbose
	}
	if c.Debug {
		return -1 // Verbose
	}
	return 0 // Normal
}
