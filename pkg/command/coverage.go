package command

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type coverageOptions struct {
	Move *struct {
		Name discordField[string] `option:"move"`
	} `option:"move"`
	Type *struct {
		Name discordField[string] `option:"type"`
	} `option:"type"`
}

type coverageResponder struct {
	autocompleteLimit int
	emojis            Emojis
}

func (resp coverageResponder) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *coverageOptions,
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

	typeString, err := resp.emojis.Emoji(typ.Name)
	if err != nil {
		return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
	}
	titleStrings = append(titleStrings, typeString)

	fields, err := efficaciesToFields(ctx, effs, true, efficacyNames{
		doubleStrong: "Super Effective (4x)",
		strong:       "Super Effective (2x)",
		neutral:      "Neutral (1x)",
		weak:         "Resists (0.5x)",
		doubleWeak:   "Resists (0.25x)",
		immune:       "Immune",
	}, resp.emojis)
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
}

func (resp coverageResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *coverageOptions,
) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch {
	case opt.Move != nil:
		if opt.Move.Name.Focused {
			s := moveSearcher{
				model:  mdl,
				prefix: opt.Move.Name.Value,
				limit:  resp.autocompleteLimit,
			}
			return searchChoices[*model.Move](ctx, s)
		}
	case opt.Type != nil:
		if opt.Type.Name.Focused {
			s := typeSearcher{
				model:  mdl,
				prefix: opt.Type.Name.Value,
				limit:  resp.autocompleteLimit,
			}
			return searchChoices[*model.Type](ctx, s)
		}
	default:
		return nil, fmt.Errorf("no recognized subcommand in focus: %w", ErrCommandFormat)
	}

	return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
}

func (builder *Builder) coverage(ctx context.Context) (Command, error) {
	resp := coverageResponder{
		autocompleteLimit: builder.config.AutocompleteLimit,
		emojis:            builder.emojis,
	}

	return command[coverageOptions]{
		handler:       resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
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
	}, nil
}
