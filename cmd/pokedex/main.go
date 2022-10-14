package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/notjagan/pokedex/pkg/bot"
	"github.com/notjagan/pokedex/pkg/config"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Read()
	if err != nil {
		log.Fatal(err)
	}

	bot, err := bot.New(ctx, *cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = bot.Run(ctx)
	defer func() {
		var err error = nil
		for err = range bot.Close() {
			log.Printf("error while shutting down bot: %v", err)
		}
		if err != nil {
			os.Exit(1)
		}
	}()
	if err != nil {
		log.Fatal(err)
	}
}
