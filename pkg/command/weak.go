package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type weakOptions struct {
	Pokemon *struct {
		Name discordField[string] `option:"pokemon"`
	} `option:"pokemon"`
	Type *struct {
		Name1 discordField[string]  `option:"type_1"`
		Name2 *discordField[string] `option:"type_2"`
	} `option:"type"`
}

type weakResponder struct {
	autocompleteLimit int
	emojis            Emojis
}

func (resp weakResponder) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *weakOptions,
) (*discordgo.InteractionResponseData, error) {
	titleStrings := make([]string, 0, 3)
	combo := mdl.NewTypeCombo()
	var sprite *discordgo.File
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

		sprite, err = pokemonSpriteFile(ctx, pokemon)
		if err != nil {
			return nil, fmt.Errorf("could not get sprite for pokemon %q: %w", pokemon.Name, err)
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

	t1, err := resp.emojis.Emoji(combo.Type1.Name)
	if err != nil {
		return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
	}
	titleStrings = append(titleStrings, t1)

	if combo.Type2 != nil {
		t2, err := resp.emojis.Emoji(combo.Type2.Name)
		if err != nil {
			return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
		}
		titleStrings = append(titleStrings, t2)
	}

	fields, err := efficaciesToFields(ctx, effs, false, efficacyNames{
		doubleStrong: "Weaknesses (4x)",
		strong:       "Weaknesses (2x)",
		weak:         "Resistances (0.5x)",
		doubleWeak:   "Resistances (0.25x)",
		immune:       "Immunities",
	}, resp.emojis)
	if err != nil {
		return nil, fmt.Errorf("could not encode type efficacies: %w", err)
	}

	embed := &discordgo.MessageEmbed{
		Title:       strings.Join(titleStrings, " "),
		Description: "Defensive type chart",
		Fields:      fields,
	}
	data := &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			embed,
		},
	}

	if sprite != nil {
		data.Files = []*discordgo.File{
			sprite,
		}
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{
			URL: fmt.Sprintf("attachment://%s", sprite.Name),
		}
	}

	return data, nil
}

func (resp weakResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *weakOptions,
) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch {
	case opt.Pokemon != nil:
		if opt.Pokemon.Name.Focused {
			s := pokemonSearcher{
				model:  mdl,
				prefix: opt.Pokemon.Name.Value,
				limit:  resp.autocompleteLimit,
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
			limit:  resp.autocompleteLimit,
		}
		return searchChoices[*model.Type](ctx, s)
	default:
		return nil, fmt.Errorf("no recognized subcommand in focus: %w", ErrCommandFormat)
	}

	return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
}

func (builder *Builder) weak(ctx context.Context) (Command, error) {
	resp := weakResponder{
		autocompleteLimit: builder.config.AutocompleteLimit,
		emojis:            builder.emojis,
	}

	return command[weakOptions]{
		handler:       resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
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
	}, nil
}
