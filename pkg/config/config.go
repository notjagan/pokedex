package config

import (
	"fmt"

	"github.com/BurntSushi/toml"
)

type CommandConfig struct {
	MoveLimit         int    `toml:"move_limit"`
	AutocompleteLimit int    `toml:"autocomplete_limit"`
	ResourceGuildID   string `toml:"resource_guild_id"`
	ResourceTimeout   int    `toml:"resource_timeout"`
}

type PokemonMetadata struct {
	MinLevel  int `toml:"min_level"`
	MaxLevel  int `toml:"max_level"`
	MoveCount int `toml:"move_count"`
}

type Config struct {
	Discord struct {
		Token         string        `toml:"token"`
		CommandConfig CommandConfig `toml:"commands"`
	} `toml:"discord"`
	DB struct {
		Path string `toml:"path"`
	} `toml:"database"`
	Pokemon struct {
		Metadata PokemonMetadata `toml:"metadata"`
	} `toml:"pokemon"`
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
