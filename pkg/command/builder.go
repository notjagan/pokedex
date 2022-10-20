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

	config     config.Config
	funcs      []commandFunc
	emojis     map[string]*discordgo.Emoji
	fieldLimit int
}

func NewBuilder(ctx context.Context, mdl *model.Model, cfg config.Config) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		Model:  mdl,
		config: cfg,
		funcs: []commandFunc{
			(*Builder).set,
			(*Builder).learnset,
			(*Builder).moves,
		},
		fieldLimit: 15,
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
		handle: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			opt *options,
		) (*discordgo.InteractionResponseData, error) {
			switch {
			case opt.Language != nil:
				err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(opt.Language.LocalizationCode))
				if err != nil {
					return nil, fmt.Errorf("error while changing language: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: "Language successfully changed.",
				}, nil

			case opt.Generation != nil:
				err := mdl.SetGenerationByID(ctx, opt.Generation.ID)
				if err != nil {
					return nil, fmt.Errorf("error while changing generation: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: "Generation successfully changed.",
				}, nil

			default:
				return nil, fmt.Errorf("missing subcommand: %w", ErrCommandFormat)
			}
		},
	}, nil
}

var ErrMissingResourceGuild = errors.New("resource guild not found")

func (builder *Builder) movesToFields(ctx context.Context, pms []model.PokemonMove) ([]*discordgo.MessageEmbedField, error) {
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

func pokemonChoices(ctx context.Context, m *model.Model, prefix string) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	ps, err := m.SearchPokemon(ctx, prefix, 25)
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
}

func (builder *Builder) checkEmojis(sess *discordgo.Session) error {
	if builder.emojis == nil {
		var guild *discordgo.Guild
		for _, g := range sess.State.Guilds {
			if g.ID == builder.config.Discord.ResourceGuildID {
				guild = g
				break
			}
		}
		if guild == nil {
			return fmt.Errorf("failed to get emotes: %w", ErrMissingResourceGuild)
		}

		builder.emojis = make(map[string]*discordgo.Emoji)
		for _, emoji := range guild.Emojis {
			builder.emojis[emoji.Name] = emoji
		}
	}

	return nil
}

func (p paginator[T]) moveButtons(hasNext bool) (*discordgo.ActionsRow, error) {
	if p.Page.Offset == 0 && !hasNext {
		return nil, nil
	}

	phome := paginator[T]{
		Options: p.Options,
		Page: Page{
			Limit:  p.Page.Limit,
			Offset: 0,
		},
	}
	homeID, err := customID(phome)
	if err != nil {
		return nil, fmt.Errorf("failed to create next button: %w", err)
	}
	homeButton := discordgo.Button{
		Style:    discordgo.PrimaryButton,
		Label:    "⏮",
		CustomID: homeID,
		Disabled: p.Page.Offset == 0,
	}

	prevOffset := p.Page.Offset - p.Page.Limit
	pprev := paginator[T]{
		Options: p.Options,
		Page: Page{
			Limit:  p.Page.Limit,
			Offset: prevOffset,
		},
	}
	prevID, err := customID(pprev)
	if err != nil {
		return nil, fmt.Errorf("failed to create previous button: %w", err)
	}
	prevButton := discordgo.Button{
		Style:    discordgo.PrimaryButton,
		Label:    "⏴",
		CustomID: prevID,
		Disabled: prevOffset < 0,
	}

	pnext := paginator[T]{
		Options: p.Options,
		Page: Page{
			Limit:  p.Page.Limit,
			Offset: p.Page.Offset + p.Page.Limit,
		},
	}
	nextID, err := customID(pnext)
	if err != nil {
		return nil, fmt.Errorf("failed to create next button: %w", err)
	}
	nextButton := discordgo.Button{
		Style:    discordgo.PrimaryButton,
		Label:    "⏵",
		CustomID: nextID,
		Disabled: !hasNext,
	}

	return &discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{
			homeButton,
			prevButton,
			nextButton,
		},
	}, nil
}

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
		paginate: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			p paginator[options],
		) (*discordgo.InteractionResponseData, error) {
			err := builder.checkEmojis(sess)
			if err != nil {
				return nil, err
			}

			pokemon, err := mdl.PokemonByName(ctx, p.Options.PokemonName.Value)
			if err != nil {
				if errors.Is(err, model.ErrWrongGeneration) {
					return &discordgo.InteractionResponseData{
						Content: "The specified Pokemon does not exist in this generation.",
					}, nil
				} else {
					return &discordgo.InteractionResponseData{
						Content: "No Pokemon found with that name.",
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

			pms, hasNext, err := pokemon.SearchPokemonMoves(ctx, defaultMethods, p.Options.MaxLevel, nil, p.Page.Limit, p.Page.Offset)
			if err != nil {
				return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
			}
			fields, err := builder.movesToFields(ctx, pms)
			if err != nil {
				return nil, fmt.Errorf("failed to convert pokemon moves to discord fields: %w", err)
			}

			embed := &discordgo.MessageEmbed{
				Title:  fmt.Sprintf("%s, %s", pokemonName, genName),
				Fields: fields,
			}
			if p.Options.MaxLevel != nil {
				embed.Description = fmt.Sprintf("Max Lv. %d", *p.Options.MaxLevel)
			}

			buttons, err := p.moveButtons(hasNext)
			if err != nil {
				return nil, fmt.Errorf("failed to generate pagination buttons: %w", err)
			}
			var components []discordgo.MessageComponent
			if buttons != nil {
				components = []discordgo.MessageComponent{buttons}
			}

			return &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
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
				return pokemonChoices(ctx, mdl, opt.PokemonName.Value)
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
		limit: &builder.fieldLimit,
	}, nil
}

func (builder *Builder) moves(ctx context.Context) (Command, error) {
	type options struct {
		PokemonName discordField[string] `option:"pokemon"`
		Level       int                  `option:"level"`
	}

	defaultMethods, err := builder.Model.LearnMethodsByName(ctx, []model.LearnMethodName{
		model.LevelUp,
	})
	if err != nil {
		return nil, fmt.Errorf("error while getting default learn methods: %w", err)
	}

	minLevel := 1.
	maxLevel := 100.

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "moves",
			Description: "Most likely moveset for a Pokemon at a given level.",
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
					Name:        "level",
					Description: "Level of the Pokemon",
					Required:    true,
					MinValue:    &minLevel,
					MaxValue:    maxLevel,
				},
			},
		},
		paginate: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			p paginator[options],
		) (*discordgo.InteractionResponseData, error) {
			err := builder.checkEmojis(sess)
			if err != nil {
				return nil, err
			}

			pokemon, err := mdl.PokemonByName(ctx, p.Options.PokemonName.Value)
			if err != nil {
				if errors.Is(err, model.ErrWrongGeneration) {
					return &discordgo.InteractionResponseData{
						Content: "The specified Pokemon does not exist in this generation.",
					}, nil
				} else {
					return &discordgo.InteractionResponseData{
						Content: "No Pokemon found with that name.",
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

			top := 4
			pms, hasNext, err := pokemon.SearchPokemonMoves(ctx, defaultMethods, &p.Options.Level, &top, p.Page.Limit, p.Page.Offset)
			if err != nil {
				return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
			}
			fields, err := builder.movesToFields(ctx, pms)
			if err != nil {
				return nil, fmt.Errorf("failed to convert pokemon moves to discord fields: %w", err)
			}

			embed := &discordgo.MessageEmbed{
				Title:       fmt.Sprintf("%s, %s", pokemonName, genName),
				Description: fmt.Sprintf("Lv. %d", p.Options.Level),
				Fields:      fields,
			}

			buttons, err := p.moveButtons(hasNext)
			if err != nil {
				return nil, fmt.Errorf("failed to generate pagination buttons: %w", err)
			}
			var components []discordgo.MessageComponent
			if buttons != nil {
				components = []discordgo.MessageComponent{buttons}
			}

			return &discordgo.InteractionResponseData{
				Embeds:     []*discordgo.MessageEmbed{embed},
				Components: components,
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
				return pokemonChoices(ctx, mdl, opt.PokemonName.Value)
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
		limit: &builder.fieldLimit,
	}, nil
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

func All(ctx context.Context, cfg config.Config) (map[string]Command, error) {
	mdl, err := model.New(ctx, cfg.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	builder := NewBuilder(ctx, mdl, cfg)
	defer builder.Close(ctx)

	return builder.all(ctx)
}
