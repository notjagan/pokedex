package main

import (
	"database/sql"
	"log"

	"github.com/BurntSushi/toml"
)

type config struct {
	Discord struct {
		Token string
	}
	DB struct {
		Path string
	}
}

const ConfigFile = "config.toml"

func hostBot(cfg config) error {
	_, err := sql.Open("sqlite3", cfg.DB.Path)
	if err != nil {
		return err
	}

	return nil
}

func main() {
	var cfg config
	_, err := toml.DecodeFile(ConfigFile, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = hostBot(cfg)
	if err != nil {
		log.Fatal(err)
	}
}
