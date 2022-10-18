package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/config"
	"github.com/notjagan/pokedex/pkg/model"
)

type commandFunc func(*Builder, context.Context) (Command, error)

type Builder struct {
	Model *model.Model

	config config.Config
	funcs  []commandFunc
	emojis map[string]*discordgo.Emoji
}

func NewBuilder(ctx context.Context, mdl *model.Model, cfg config.Config) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		Model:  mdl,
		config: cfg,
		funcs: []commandFunc{
			(*Builder).set,
			(*Builder).learnset,
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

var ErrNoEmoji = errors.New("no matching emoji")

func (builder *Builder) ToEmojiString(name string) (string, error) {
	if builder.emojis == nil {
		return "", fmt.Errorf("emojis map not set for builder: %w", ErrNoEmoji)
	}

	emoji1, ok := builder.emojis[name+"1"]
	if !ok {
		return "", fmt.Errorf("could not find first emoji for item %q: %w", name, ErrNoEmoji)
	}

	emoji2, ok := builder.emojis[name+"2"]
	if !ok {
		return "", fmt.Errorf("could not find second emoji for item %q: %w", name, ErrNoEmoji)
	}

	return fmt.Sprintf("<:%v:%v><:%v:%v>", emoji1.Name, emoji1.ID, emoji2.Name, emoji2.ID), nil
}

var ErrCommandFormat = errors.New("invalid command format")

func (builder *Builder) set(ctx context.Context) (Command, error) {
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
		return nil, fmt.Errorf("error while getting min gen for set command: %w", err)
	}
	minGen := float64(gen.ID)

	gen, err = builder.Model.LatestGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting max gen for set command: %w", err)
	}
	maxGen := float64(gen.ID)

	langs, err := builder.Model.AllLanguages(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting available language options: %w", err)
	}

	langChoices := make([]*discordgo.ApplicationCommandOptionChoice, len(langs))
	for i, lang := range langs {
		name, err := lang.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while localizing language options: %w", err)
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
		handler: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			opt *options,
		) (*discordgo.InteractionResponse, error) {
			switch {
			case opt.Language != nil:
				err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(opt.Language.LocalizationCode))
				if err != nil {
					return nil, fmt.Errorf("error while changing language: %w", err)
				}

				return &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Language successfully changed.",
					},
				}, nil

			case opt.Generation != nil:
				err := mdl.SetGenerationByID(ctx, opt.Generation.ID)
				if err != nil {
					return nil, fmt.Errorf("error while changing generation: %w", err)
				}

				return &discordgo.InteractionResponse{
					Type: discordgo.InteractionResponseChannelMessageWithSource,
					Data: &discordgo.InteractionResponseData{
						Content: "Generation successfully changed.",
					},
				}, nil

			default:
				return nil, fmt.Errorf("missing subcommand: %w", ErrCommandFormat)
			}
		},
	}, nil
}

var ErrMissingResourceGuild = errors.New("resource guild not found")

func (builder *Builder) learnset(ctx context.Context) (Command, error) {
	type options struct {
		PokemonName discordField[string] `option:"pokemon"`
		MaxLevel    *int                 `option:"max_level"`
	}

	defaultMethods, err := builder.Model.LearnMethodsByName(ctx, []model.LearnMethodName{
		model.LevelUp,
	})
	if err != nil {
		return nil, fmt.Errorf("error while getting default learn methods: %w", err)
	}

	movesToFields := func(ctx context.Context, pms []model.PokemonMove) ([]*discordgo.MessageEmbedField, error) {
		fields := make([]*discordgo.MessageEmbedField, len(pms))
		for i, pm := range pms {
			values := make([]string, 0, 5)

			move, err := pm.Move(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while getting move data for pokemon move: %w", err)
			}

			name, err := move.LocalizedName(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get localized name for move %q: %w", move.Name, err)
			}

			typ, err := move.Type(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while getting type for move %q: %w", move.Name, err)
			}
			if !typ.IsUnknown() {
				typeString, err := builder.ToEmojiString(typ.Name)
				if err != nil {
					return nil, fmt.Errorf("error while constructing type emoji string for move %q: %w", move.Name, err)
				}
				values = append(values, typeString)
			}

			class, err := move.DamageClass(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while getting damage class for move %q: %w", move.Name, err)
			}
			classString, err := builder.ToEmojiString(class.Name)
			if err != nil {
				return nil, fmt.Errorf("error while constructing type emoji string for move %q: %w", move.Name, err)
			}
			values = append(values, classString)

			if move.Power != nil {
				values = append(values, fmt.Sprintf("%d `POWER`", *move.Power))
			}

			if move.Accuracy != nil {
				values = append(values, fmt.Sprintf("%d%%", *move.Accuracy))
			}

			values = append(values, fmt.Sprintf("%d `PP`", move.PP))

			fields[i] = &discordgo.MessageEmbedField{
				Name:  fmt.Sprintf("Lv. %-2d ▸ %s", pm.Level, name),
				Value: strings.Join(values, " ▸ "),
			}
		}

		return fields, nil
	}

	minLevel := 1.
	maxLevel := 100.

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "learnset",
			Description: "Learnset for a given Pokemon.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "pokemon",
					Description:  "Name of the Pokemon",
					Required:     true,
					Autocomplete: true,
				},
				{
					Type:        discordgo.ApplicationCommandOptionInteger,
					Name:        "max_level",
					Description: "Level cap for learnset",
					Required:    false,
					MinValue:    &minLevel,
					MaxValue:    maxLevel,
				},
			},
		},
		handler: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			opt *options,
		) (*discordgo.InteractionResponse, error) {
			if builder.emojis == nil {
				var guild *discordgo.Guild
				for _, g := range sess.State.Guilds {
					if g.ID == builder.config.Discord.ResourceGuildID {
						guild = g
						break
					}
				}
				if guild == nil {
					return nil, fmt.Errorf("failed to get emotes: %w", ErrMissingResourceGuild)
				}

				builder.emojis = make(map[string]*discordgo.Emoji)
				for _, emoji := range guild.Emojis {
					builder.emojis[emoji.Name] = emoji
				}
			}

			pokemon, err := mdl.PokemonByName(ctx, opt.PokemonName.Value)
			if err != nil {
				if errors.Is(err, model.ErrWrongGeneration) {
					return &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "The specified Pokemon does not exist in this generation.",
						},
					}, nil
				} else {
					return &discordgo.InteractionResponse{
						Type: discordgo.InteractionResponseChannelMessageWithSource,
						Data: &discordgo.InteractionResponseData{
							Content: "No Pokemon found with that name.",
						},
					}, nil
				}
			}

			pokemonName, err := pokemon.LocalizedName(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get localized name for pokemon %q: %w", pokemon.Name, err)
			}

			if mdl.Generation == nil {
				return nil, fmt.Errorf("could not get localized name for generation: %w", model.ErrUnsetGeneration)
			}
			genName, err := mdl.Generation.LocalizedName(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get localized name for generation %d: %w", mdl.Generation.ID, err)
			}

			pms, err := pokemon.PokemonMoves(ctx, defaultMethods, opt.MaxLevel, nil)
			if err != nil {
				return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
			}
			fields, err := movesToFields(ctx, pms)
			if err != nil {
				return nil, fmt.Errorf("failed to convert pokemon moves to discord fields: %w", err)
			}

			return &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Embeds: []*discordgo.MessageEmbed{
						{
							Title:  fmt.Sprintf("%s, %s", pokemonName, genName),
							Fields: fields,
						},
					},
				},
			}, nil
		},
		autocomplete: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			opt *options,
		) ([]*discordgo.ApplicationCommandOptionChoice, error) {
			switch {
			case opt.PokemonName.Focused:
				ps, err := mdl.SearchPokemon(ctx, opt.PokemonName.Value, 25)
				if err != nil {
					return nil, fmt.Errorf("error while searching for matching pokemon: %w", err)
				}

				choices := make([]*discordgo.ApplicationCommandOptionChoice, len(ps))
				for i, pokemon := range ps {
					name, err := pokemon.LocalizedName(ctx)
					if err != nil {
						return nil, fmt.Errorf("error while getting localized name for pokemon %q: %w", pokemon.Name, err)
					}

					choices[i] = &discordgo.ApplicationCommandOptionChoice{
						Name:  name,
						Value: pokemon.Name,
					}
				}

				return choices, nil
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
	}, nil
}

func (builder *Builder) all(ctx context.Context) ([]Command, error) {
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

func All(ctx context.Context, cfg config.Config) ([]Command, error) {
	mdl, err := model.New(ctx, cfg.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	builder := NewBuilder(ctx, mdl, cfg)
	defer builder.Close(ctx)

	return builder.all(ctx)
}
