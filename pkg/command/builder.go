package command

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/config"
	"github.com/notjagan/pokedex/pkg/model"
)

type Emojis map[string]*discordgo.Emoji

type Builder struct {
	model *model.Model

	config   config.CommandConfig
	metadata config.PokemonMetadata
	funcs    []func(*Builder, context.Context) (Command, error)
	emojis   Emojis
}

func NewBuilder(ctx context.Context, mdl *model.Model, cfg config.Config, emojis Emojis) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		model:    mdl,
		config:   cfg.Discord.CommandConfig,
		metadata: cfg.Pokemon.Metadata,
		funcs: []func(*Builder, context.Context) (Command, error){
			(*Builder).language,
			(*Builder).version,
			(*Builder).learnset,
			(*Builder).moves,
			(*Builder).weak,
			(*Builder).coverage,
			(*Builder).dex,
		},
		emojis: emojis,
	}
}

func (builder *Builder) Close(ctx context.Context) error {
	err := builder.model.Close()
	if err != nil {
		return fmt.Errorf("error while closing model for command builder: %w", err)
	}

	return nil
}

func (builder *Builder) all(ctx context.Context) (map[string]Command, error) {
	commands := make(map[string]Command, len(builder.funcs))

	for _, f := range builder.funcs {
		cmd, err := f(builder, ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating command: %w", err)
		}
		commands[cmd.Name()] = cmd
	}

	return commands, nil
}

func All(ctx context.Context, cfg config.Config, emojis Emojis) (map[string]Command, error) {
	mdl, err := model.New(ctx, cfg.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	builder := NewBuilder(ctx, mdl, cfg, emojis)
	defer builder.Close(ctx)

	return builder.all(ctx)
}
