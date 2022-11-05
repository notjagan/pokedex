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
	model *model.Model

	config            config.Config
	funcs             []commandFunc
	emojis            map[string]*discordgo.Emoji
	moveLimit         int
	autocompleteLimit int
}

func NewBuilder(ctx context.Context, mdl *model.Model, cfg config.Config) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		model:  mdl,
		config: cfg,
		funcs: []commandFunc{
			(*Builder).language,
			(*Builder).version,
			(*Builder).learnset,
			(*Builder).moves,
			(*Builder).weak,
			(*Builder).coverage,
		},
		moveLimit:         15,
		autocompleteLimit: 25,
	}
}

func (builder *Builder) Close(ctx context.Context) error {
	err := builder.model.Close()
	if err != nil {
		return fmt.Errorf("error while closing model for command builder: %w", err)
	}

	return nil
}

var ErrNoEmoji = errors.New("no matching emoji")

func (builder *Builder) ToEmojiString(sess *discordgo.Session, name string) (string, error) {
	err := builder.checkEmojis(sess)
	if err != nil {
		return "", fmt.Errorf("could not get custom emojis: %w", err)
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

func (builder *Builder) language(ctx context.Context) (Command, error) {
	type options struct {
		LocalizationCode *string `option:"language"`
	}

	s := languageSearcher{model: builder.model}
	langChoices, err := searchChoices[*model.Language](ctx, s)
	if err != nil {
		return nil, fmt.Errorf("could not get available language choices: %w", err)
	}

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "language",
			Description: "Get/set the the current Pokedex language.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "language",
					Description: "Language to set Pokedex to",
					Required:    false,
					Choices:     langChoices,
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
			if opt.LocalizationCode == nil {
				name, err := mdl.Language.LocalizedName(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not localize current language name: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Language is currently %q.", name),
				}, nil
			} else {
				err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(*opt.LocalizationCode))
				if err != nil {
					return nil, fmt.Errorf("error while changing language: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: "Language successfully changed.",
				}, nil
			}
		},
	}, nil
}

func (builder *Builder) version(ctx context.Context) (Command, error) {
	type options struct {
		Name *discordField[string] `option:"version"`
	}

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "version",
			Description: "Get/set the current Pokedex game version.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "version",
					Description:  "Game version to pull data from",
					Required:     false,
					Autocomplete: true,
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
			if opt.Name == nil {
				name, err := mdl.Version.LocalizedName(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not localize current version name: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: fmt.Sprintf("Currently using Pokemon %s.", name),
				}, nil
			} else {
				err := mdl.SetVersionByName(ctx, opt.Name.Value)
				if err != nil {
					return nil, fmt.Errorf("error while changing version: %w", err)
				}

				return &discordgo.InteractionResponseData{
					Content: "Version successfully changed.",
				}, nil
			}
		},
		autocomplete: func(
			ctx context.Context,
			mdl *model.Model,
			sess *discordgo.Session,
			interaction *discordgo.InteractionCreate,
			opt *options,
		) ([]*discordgo.ApplicationCommandOptionChoice, error) {
			switch {
			case opt.Name != nil && opt.Name.Focused:
				s := versionSearcher{
					model:  mdl,
					prefix: opt.Name.Value,
					limit:  builder.autocompleteLimit,
				}
				return searchChoices[*model.Version](ctx, s)
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
	}, nil
}

var ErrMissingResourceGuild = errors.New("resource guild not found")

func (builder *Builder) movesToFields(ctx context.Context, sess *discordgo.Session, pms []model.PokemonMove) ([]*discordgo.MessageEmbedField, error) {
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
			typeString, err := builder.ToEmojiString(sess, typ.Name)
			if err != nil {
				return nil, fmt.Errorf("error while constructing type emoji string for move %q: %w", move.Name, err)
			}
			values = append(values, typeString)
		}

		class, err := move.DamageClass(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting damage class for move %q: %w", move.Name, err)
		}
		classString, err := builder.ToEmojiString(sess, class.Name)
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

		if move.PP != nil {
			values = append(values, fmt.Sprintf("%d `PP`", *move.PP))
		}

		fields[i] = &discordgo.MessageEmbedField{
			Name:  fmt.Sprintf("Lv. %-2d ▸ %s", pm.Level, name),
			Value: strings.Join(values, " ▸ "),
		}
	}

	return fields, nil
}

func searchChoices[T model.Localizer](ctx context.Context, s searcher[T]) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	results, err := s.Search(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while searching for matching pokemon: %w", err)
	}

	choices := make([]*discordgo.ApplicationCommandOptionChoice, len(results))
	for i, res := range results {
		name, err := res.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting localized name for resource: %w", err)
		}

		choices[i] = &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: s.Value(res),
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
		EggMoves    *bool                `option:"egg_moves"`
	}

	defaultMethodNames := []model.LearnMethodName{
		model.LevelUp,
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
				{
					Type:        discordgo.ApplicationCommandOptionBoolean,
					Name:        "egg_moves",
					Description: "Include egg moves",
					Required:    false,
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

			if mdl.Version == nil {
				return nil, fmt.Errorf("could not get localized name for version: %w", model.ErrUnsetVersion)
			}
			gen, err := mdl.Version.Generation(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get generation for model version: %w", err)
			}
			genName, err := gen.LocalizedName(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get localized name for generation %d: %w", gen.ID, err)
			}

			methodNames := defaultMethodNames[:]
			if p.Options.EggMoves != nil && *p.Options.EggMoves {
				methodNames = append(methodNames, model.Egg)
			}
			methods, err := mdl.LearnMethodsByName(ctx, methodNames)
			if err != nil {
				return nil, fmt.Errorf("failed to get learn methods: %w", err)
			}

			pms, hasNext, err := pokemon.SearchPokemonMoves(ctx, methods, p.Options.MaxLevel, nil, p.Page.Limit, p.Page.Offset)
			if err != nil {
				return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
			}
			fields, err := builder.movesToFields(ctx, sess, pms)
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
				s := pokemonSearcher{
					model:  mdl,
					prefix: opt.PokemonName.Value,
					limit:  builder.autocompleteLimit,
				}
				return searchChoices[*model.Pokemon](ctx, s)
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
		limit: &builder.moveLimit,
	}, nil
}

func (builder *Builder) moves(ctx context.Context) (Command, error) {
	type options struct {
		PokemonName discordField[string] `option:"pokemon"`
		Level       int                  `option:"level"`
	}

	defaultMethods, err := builder.model.LearnMethodsByName(ctx, []model.LearnMethodName{
		model.LevelUp,
	})
	if err != nil {
		return nil, fmt.Errorf("error while getting default learn methods: %w", err)
	}

	minLevel := 1.
	maxLevel := 100.
	numMoves := 4

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

			if mdl.Version == nil {
				return nil, fmt.Errorf("could not get localized name for version: %w", model.ErrUnsetVersion)
			}
			gen, err := mdl.Version.Generation(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get generation for model version: %w", err)
			}
			genName, err := gen.LocalizedName(ctx)
			if err != nil {
				return nil, fmt.Errorf("could not get localized name for generation %d: %w", gen.ID, err)
			}

			pms, hasNext, err := pokemon.SearchPokemonMoves(ctx, defaultMethods, &p.Options.Level, &numMoves, p.Page.Limit, p.Page.Offset)
			if err != nil {
				return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
			}
			fields, err := builder.movesToFields(ctx, sess, pms)
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
				s := pokemonSearcher{
					model:  mdl,
					prefix: opt.PokemonName.Value,
					limit:  builder.autocompleteLimit,
				}
				return searchChoices[*model.Pokemon](ctx, s)
			default:
				return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
			}
		},
		limit: &builder.moveLimit,
	}, nil
}

type efficacyNames struct {
	doubleStrong string
	strong       string
	neutral      string
	weak         string
	doubleWeak   string
	immune       string
}

func (builder *Builder) efficaciesToFields(
	ctx context.Context,
	sess *discordgo.Session,
	effs []model.TypeEfficacy,
	includeAll bool,
	names efficacyNames,
) ([]*discordgo.MessageEmbedField, error) {
	n := len(effs)
	doubleStrengths := make([]string, 0, n)
	strengths := make([]string, 0, n)
	neutrals := make([]string, 0, n)
	weaks := make([]string, 0, n)
	doubleWeaks := make([]string, 0, n)
	immunes := make([]string, 0, n)

	for _, te := range effs {
		typ, err := te.OpposingType(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to encode type efficacies: %w", err)
		}
		emoji, err := builder.ToEmojiString(sess, typ.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to get emoji for type efficacies: %w", err)
		}

		switch te.EfficacyLevel() {
		case model.DoubleSuperEffective:
			doubleStrengths = append(doubleStrengths, emoji)
		case model.SuperEffective:
			strengths = append(strengths, emoji)
		case model.NormalEffective:
			neutrals = append(neutrals, emoji)
		case model.NotVeryEffective:
			weaks = append(weaks, emoji)
		case model.DoubleNotVeryEffective:
			doubleWeaks = append(doubleWeaks, emoji)
		case model.Immune:
			immunes = append(immunes, emoji)
		default:
			return nil, fmt.Errorf("unexpected type efficacy level: %w", ErrUnrecognizedInteraction)
		}
	}

	fields := make([]*discordgo.MessageEmbedField, 0, 6)
	if len(doubleStrengths) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.doubleStrong,
			Value: strings.Join(doubleStrengths, " "),
		})
	}

	if len(strengths) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.strong,
			Value: strings.Join(strengths, " "),
		})
	} else if includeAll {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.strong,
			Value: "_None_",
		})
	}

	if includeAll {
		if len(neutrals) > 0 {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  names.neutral,
				Value: strings.Join(neutrals, " "),
			})
		} else {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:  names.neutral,
				Value: "_None_",
			})
		}
	}

	if len(weaks) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.weak,
			Value: strings.Join(weaks, " "),
		})
	} else if includeAll {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.weak,
			Value: "_None_",
		})
	}

	if len(doubleWeaks) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.doubleWeak,
			Value: strings.Join(doubleWeaks, " "),
		})
	}

	if len(immunes) > 0 {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.immune,
			Value: strings.Join(immunes, " "),
		})
	} else if includeAll {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:  names.immune,
			Value: "_None_",
		})
	}

	return fields, nil
}

func (builder *Builder) weak(ctx context.Context) (Command, error) {
	type options struct {
		Pokemon *struct {
			Name discordField[string] `option:"pokemon"`
		} `option:"pokemon"`
		Type *struct {
			Name1 discordField[string]  `option:"type_1"`
			Name2 *discordField[string] `option:"type_2"`
		} `option:"type"`
	}

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "weak",
			Description: "View type chart against a defending Pokemon/type combination.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "pokemon",
					Description: "View type chart against a defending Pokemon",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "pokemon",
							Description:  "Name of the Pokemon",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "type",
					Description: "View type chart against a defending type (combination)",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "type_1",
							Description:  "Name of the first type",
							Required:     true,
							Autocomplete: true,
						},
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "type_2",
							Description:  "Name of the second type",
							Required:     false,
							Autocomplete: true,
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
			titleStrings := make([]string, 0, 3)
			combo := mdl.NewTypeCombo()
			switch {
			case opt.Pokemon != nil:
				pokemon, err := mdl.PokemonByName(ctx, opt.Pokemon.Name.Value)
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

				name, err := pokemon.LocalizedName(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not get localized name for pokemon %q: %w", pokemon.Name, err)
				}
				titleStrings = append(titleStrings, name)

				combo, err = pokemon.TypeCombo(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not get type combo for pokemon: %w", err)
				}
			case opt.Type != nil:
				typ1, err := mdl.TypeByName(ctx, opt.Type.Name1.Value)
				if err != nil {
					return nil, fmt.Errorf("could not get first type by name: %w", err)
				}
				combo.Type1 = typ1

				if opt.Type.Name2 != nil {
					typ2, err := mdl.TypeByName(ctx, opt.Type.Name2.Value)
					if err != nil {
						return nil, fmt.Errorf("could not get second type by name: %w", err)
					}
					combo.Type2 = typ2
				}
			default:
				return nil, fmt.Errorf("unrecognized subcommand for command \"weak\": %w", ErrCommandFormat)
			}

			effs, err := combo.DefendingEfficacies(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while get efficacies for type combo: %w", err)
			}

			t1, err := builder.ToEmojiString(sess, combo.Type1.Name)
			if err != nil {
				return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
			}
			titleStrings = append(titleStrings, t1)

			if combo.Type2 != nil {
				t2, err := builder.ToEmojiString(sess, combo.Type2.Name)
				if err != nil {
					return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
				}
				titleStrings = append(titleStrings, t2)
			}

			fields, err := builder.efficaciesToFields(ctx, sess, effs, false, efficacyNames{
				doubleStrong: "Weaknesses (4x)",
				strong:       "Weaknesses (2x)",
				weak:         "Resistances (0.5x)",
				doubleWeak:   "Resistances (0.25x)",
				immune:       "Immunities",
			})
			if err != nil {
				return nil, fmt.Errorf("could not encode type efficacies: %w", err)
			}

			return &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       strings.Join(titleStrings, " "),
						Description: "Defensive type chart",
						Fields:      fields,
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
			case opt.Pokemon != nil:
				if opt.Pokemon.Name.Focused {
					s := pokemonSearcher{
						model:  mdl,
						prefix: opt.Pokemon.Name.Value,
						limit:  builder.autocompleteLimit,
					}
					return searchChoices[*model.Pokemon](ctx, s)
				}
			case opt.Type != nil:
				var prefix string
				switch {
				case opt.Type.Name1.Focused:
					prefix = opt.Type.Name1.Value
				case opt.Type.Name2 != nil && opt.Type.Name2.Focused:
					prefix = opt.Type.Name2.Value
				default:
					return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
				}

				s := typeSearcher{
					model:  mdl,
					prefix: prefix,
					limit:  builder.autocompleteLimit,
				}
				return searchChoices[*model.Type](ctx, s)
			default:
				return nil, fmt.Errorf("no recognized subcommand in focus: %w", ErrCommandFormat)
			}

			return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
		},
	}, nil
}

func (builder *Builder) coverage(ctx context.Context) (Command, error) {
	type options struct {
		Move *struct {
			Name discordField[string] `option:"move"`
		} `option:"move"`
		Type *struct {
			Name discordField[string] `option:"type"`
		} `option:"type"`
	}

	return command[options]{
		applicationCommand: &discordgo.ApplicationCommand{
			Name:        "coverage",
			Description: "View type chart for an attacking move/type combination.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "move",
					Description: "View type chart for an attacking move",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "move",
							Description:  "Name of the move",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "type",
					Description: "View type chart for an attacking type",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "type",
							Description:  "Name of the type",
							Required:     true,
							Autocomplete: true,
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
			titleStrings := make([]string, 0, 2)
			var typ *model.Type
			switch {
			case opt.Move != nil:
				move, err := mdl.MoveByName(ctx, opt.Move.Name.Value)
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

				name, err := move.LocalizedName(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not get localized name for move %q: %w", move.Name, err)
				}
				titleStrings = append(titleStrings, name)

				typ, err = move.Type(ctx)
				if err != nil {
					return nil, fmt.Errorf("could not get type for move: %w", err)
				}
			case opt.Type != nil:
				var err error
				typ, err = mdl.TypeByName(ctx, opt.Type.Name.Value)
				if err != nil {
					return nil, fmt.Errorf("could not get first type by name: %w", err)
				}
			default:
				return nil, fmt.Errorf("unrecognized subcommand for command \"weak\": %w", ErrCommandFormat)
			}

			effs, err := typ.AttackingEfficacies(ctx)
			if err != nil {
				return nil, fmt.Errorf("error while get efficacies for type combo: %w", err)
			}

			typeString, err := builder.ToEmojiString(sess, typ.Name)
			if err != nil {
				return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
			}
			titleStrings = append(titleStrings, typeString)

			fields, err := builder.efficaciesToFields(ctx, sess, effs, true, efficacyNames{
				doubleStrong: "Super Effective (4x)",
				strong:       "Super Effective (2x)",
				neutral:      "Neutral (1x)",
				weak:         "Resists (0.5x)",
				doubleWeak:   "Resists (0.25x)",
				immune:       "Immune",
			})
			if err != nil {
				return nil, fmt.Errorf("could not encode type efficacies: %w", err)
			}

			return &discordgo.InteractionResponseData{
				Embeds: []*discordgo.MessageEmbed{
					{
						Title:       strings.Join(titleStrings, " "),
						Description: "Offensive type chart",
						Fields:      fields,
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
			case opt.Move != nil:
				if opt.Move.Name.Focused {
					s := moveSearcher{
						model:  mdl,
						prefix: opt.Move.Name.Value,
						limit:  builder.autocompleteLimit,
					}
					return searchChoices[*model.Move](ctx, s)
				}
			case opt.Type != nil:
				if opt.Type.Name.Focused {
					s := typeSearcher{
						model:  mdl,
						prefix: opt.Type.Name.Value,
						limit:  builder.autocompleteLimit,
					}
					return searchChoices[*model.Type](ctx, s)
				}
			default:
				return nil, fmt.Errorf("no recognized subcommand in focus: %w", ErrCommandFormat)
			}

			return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
		},
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
