package config

type Modifier interface {
	Recommend() *Config
	Override() *Config
}

// Merge will combine all the configs from the channels passed in. Last channel
// will take precedent. If a channel returns nil for recommend or override, it will be skipped
func Merge(inputConfig *Config, providers ...Modifier) *Config {
	cfg := &Config{Config: map[string]interface{}{}}

	for _, channel := range providers {
		if channel == nil {
			continue
		}
		if c := channel.Recommend(); c != nil {
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
		if c := channel.Override(); c != nil {
			cfg.Merge(c)
		}
	}

	return cfg
}
