package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type commandFunc func(*Builder, context.Context) (Command, error)

type Builder struct {
	Model *model.Model

	funcs []commandFunc
}

func NewBuilder(ctx context.Context, mdl *model.Model) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		Model: mdl,
		funcs: []commandFunc{
			Set,
		},
	}
}

func (builder *Builder) Close(ctx context.Context) error {
	err := builder.Model.Close()
	if err != nil {
		return fmt.Errorf("error while closing model for command builder: %w", err)
	}

	return nil
}

var ErrCommandFormat = errors.New("invalid command format")

func Set(builder *Builder, ctx context.Context) (Command, error) {
	type options struct {
		Language *struct {
			LocalizationCode string `option:"language"`
		} `option:"language"`
		Generation *struct {
			ID int `option:"generation_number"`
		} `option:"generation"`
	}

	gen, err := builder.Model.EarliestGeneration(ctx)
	if err != nil {
		return command[options]{}, fmt.Errorf("error while getting min gen for set command: %w", err)
	}
	minGen := float64(gen.ID)

	gen, err = builder.Model.LatestGeneration(ctx)
	if err != nil {
		return command[options]{}, fmt.Errorf("error while getting max gen for set command: %w", err)
	}
	maxGen := float64(gen.ID)

	langs, err := builder.Model.AllLanguages(ctx)
	if err != nil {
		return command[options]{}, fmt.Errorf("error while getting available language options: %w", err)
	}

	langChoices := make([]*discordgo.ApplicationCommandOptionChoice, len(langs))
	for i, lang := range langs {
		name, err := lang.LocalizedName(ctx)
		if err != nil {
			return command[options]{}, fmt.Errorf("error while localizing language options: %w", err)
		}
		langChoices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: lang.ISO639,
		}
	}

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "set",
			Description: "Set a server-wide configuration value for the Pokedex.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "generation",
					Description: "Set Pokemon generation",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionInteger,
							Name:        "generation_number",
							Description: "Game generation to pull data from",
							Required:    true,
							MinValue:    &minGen,
							MaxValue:    maxGen,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "language",
					Description: "Set language for data",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:        discordgo.ApplicationCommandOptionString,
							Name:        "language",
							Description: "Language to use",
							Required:    true,
							Choices:     langChoices,
						},
					},
				},
			},
		},
		handler: func(ctx context.Context, mdl *model.Model, sess *discordgo.Session, interaction *discordgo.InteractionCreate, opt options) error {
			if opt.Language != nil {
				err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(opt.Language.LocalizationCode))
				if err != nil {
					return fmt.Errorf("error while changing language: %w", err)
				}

				err = sess.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Language successfully changed.",
					},
				})
				if err != nil {
					return fmt.Errorf("error while responding to command: %w", err)
				}
			} else if opt.Generation != nil {
				err := mdl.SetGenerationByID(ctx, opt.Generation.ID)
				if err != nil {
					return fmt.Errorf("error while changing generation: %w", err)
				}

				err = sess.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Generation successfully changed.",
					},
				})
				if err != nil {
					return fmt.Errorf("error while responding to command: %w", err)
				}
			} else {
				return fmt.Errorf("missing subcommand: %w", ErrCommandFormat)
			}

			return nil
		},
	}, nil
}

func (builder *Builder) All(ctx context.Context) ([]Command, error) {
	commands := make([]Command, len(builder.funcs))

	for i, f := range builder.funcs {
		cmd, err := f(builder, ctx)
		if err != nil {
			return nil, fmt.Errorf("error while creating command: %w", err)
		}
		commands[i] = cmd
	}

	return commands, nil
}

func All(ctx context.Context, dbPath string) ([]Command, error) {
	mdl, err := model.New(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	builder := NewBuilder(ctx, mdl)
	defer builder.Close(ctx)
	return builder.All(ctx)
}
