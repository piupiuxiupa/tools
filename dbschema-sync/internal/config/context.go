package config

import "context"

type contextKey string

const configKey contextKey = "config"

// WithConfig adds config to context
func WithConfig(ctx context.Context, cfg *Config) context.Context {
	return context.WithValue(ctx, configKey, cfg)
}

// GetConfig retrieves config from context
func GetConfig(ctx context.Context) *Config {
	if cfg, ok := ctx.Value(configKey).(*Config); ok {
		return cfg
	}
	return DefaultConfig()
}
