package config

type Config struct {
	Discord struct {
		Token string `toml:"token"`
	} `toml:"discord"`
	DB struct {
		Path string `toml:"path"`
	} `toml:"database"`
}
