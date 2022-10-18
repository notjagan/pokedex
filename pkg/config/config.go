package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Discord struct {
		Token           string `toml:"token"`
		ResourceGuildID string `toml:"resource_guild_id"`
	} `toml:"discord"`
	DB struct {
		Path string `toml:"path"`
	} `toml:"database"`
}

const ConfigFile = "config.toml"

func Read() (*Config, error) {
	var cfg Config
	_, err := toml.DecodeFile(ConfigFile, &cfg)
	if err != nil {
		return nil, fmt.Errorf("error while decoding configuration from %q: %w", ConfigFile, err)
	}

	return &cfg, nil
}
