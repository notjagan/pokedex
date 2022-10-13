package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/BurntSushi/toml"
	_ "github.com/mattn/go-sqlite3"
	"github.com/notjagan/pokedex/pkg/bot"
	"github.com/notjagan/pokedex/pkg/config"
)

const ConfigFile = "config.toml"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var cfg config.Config
	_, err := toml.DecodeFile(ConfigFile, &cfg)
	if err != nil {
		log.Fatal(err)
	}

	bot, err := bot.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = bot.Run(ctx)
	if err != nil {
		log.Fatal(err)
	}
}
