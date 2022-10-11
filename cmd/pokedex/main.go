package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	_ "github.com/mattn/go-sqlite3"
)

type config struct {
	Discord struct {
		Token string `toml:"token"`
	} `toml:"discord"`
	DB struct {
		Path string `toml:"path"`
	} `toml:"database"`
}

const ConfigFile = "config.toml"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var cfg config
	_, err := toml.DecodeFile(ConfigFile, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	_ = ctx
}
