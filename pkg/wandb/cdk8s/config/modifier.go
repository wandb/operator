package config

// Modifier is used to override or recommend a config for the user. It will
// modify or add defaults to the users config.
type Modifier interface {
	// Recommend will return a config that will be applied for the users config.
	// This means the user can override any values set here cfg is the current
	// state of the config before applying these changes
	Recommend(cfg *Config) *Config
	// Override will return a config that will be applied and override the users
	// specified config. cfg is the current state of the config before applying
	// these changes
	Override(cfg *Config) *Config
}

// Merge will combine all the configs from the channels passed in. Last channel
// will take precedent. If a channel returns nil for recommend or override, it
// will be skipped
func Merge(inputConfig *Config, providers ...Modifier) *Config {
	cfg := &Config{Config: map[string]interface{}{}}

	for _, channel := range providers {
		if channel == nil {
			continue
		}
		if c := channel.Recommend(cfg); c != nil {
			cfg.Merge(c)
		}
	}

	if inputConfig != nil {
		cfg.Merge(inputConfig)
	}

	for _, channel := range providers {
		if channel == nil {
			continue
		}
		if c := channel.Override(cfg); c != nil {
			cfg.Merge(c)
		}
	}

	return cfg
}
