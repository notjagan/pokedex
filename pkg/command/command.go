package command

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type Command interface {
	ApplicationCommand() *discordgo.ApplicationCommand
	CallHandler(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
	Name() string
}

type command[S any] struct {
	applicationCommand *discordgo.ApplicationCommand
	handler            func(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate, S) error
}

func (cmd command[S]) ApplicationCommand() *discordgo.ApplicationCommand {
	return cmd.applicationCommand
}

func (cmd command[S]) Name() string {
	return cmd.applicationCommand.Name
}

func (cmd command[S]) CallHandler(ctx context.Context, mdl *model.Model, sess *discordgo.Session, interaction *discordgo.InteractionCreate) error {
	var structure S
	err := decodeOptions(interaction.ApplicationCommandData().Options, &structure)
	if err != nil {
		return fmt.Errorf("error while decoding options for command: %w", err)
	}

	err = cmd.handler(ctx, mdl, sess, interaction, structure)
	if err != nil {
		return fmt.Errorf("error while calling handler: %w", err)
	}

	return nil
}

var ErrDecodeOption = errors.New("error while decoding options")

func decodeOptions(options []*discordgo.ApplicationCommandInteractionDataOption, pointer any) error {
	t := reflect.TypeOf(pointer)
	if t.Kind() != reflect.Pointer {
		return fmt.Errorf("cannot populate values for non-pointer: %w", ErrDecodeOption)
	}
	if t.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("cannot assign fields to non-struct element: %w", ErrDecodeOption)
	}
	if reflect.ValueOf(pointer).IsNil() {
		return fmt.Errorf("pointer to structure must not be nil: %w", ErrDecodeOption)
	}

	structure := reflect.ValueOf(pointer).Elem()
	m := make(map[string]reflect.Value, structure.NumField())
	for i := 0; i < structure.NumField(); i++ {
		field := structure.Field(i)
		fieldt := t.Elem().Field(i)
		if !field.CanSet() {
			return fmt.Errorf("field %q cannot be set: %w", fieldt.Name, ErrDecodeOption)
		}
		m[fieldt.Tag.Get("option")] = field
	}

	for _, option := range options {
		field, ok := m[option.Name]
		if !ok {
			return fmt.Errorf("unexpected option name %q: %w", option.Name, ErrDecodeOption)
		}

		switch option.Type {
		case discordgo.ApplicationCommandOptionString:
			if field.Kind() == reflect.String {
				field.SetString(option.StringValue())
				continue
			}
		case discordgo.ApplicationCommandOptionInteger:
			if field.Kind() == reflect.Int {
				field.SetInt(option.IntValue())
				continue
			}
		case discordgo.ApplicationCommandOptionBoolean:
			if field.Kind() == reflect.Bool {
				field.SetBool(option.BoolValue())
				continue
			}
		case discordgo.ApplicationCommandOptionSubCommand:
			if field.Kind() == reflect.Pointer && field.Type().Elem().Kind() == reflect.Struct {
				ptr := reflect.New(field.Type().Elem())
				field.Set(ptr)

				err := decodeOptions(option.Options, ptr.Interface())
				if err != nil {
					return fmt.Errorf("error while decoding options for subcommand %q: %w", option.Name, err)
				}

				continue
			}
		default:
			return fmt.Errorf("unsupported type %q for option %q: %w", option.Type, option.Name, ErrDecodeOption)
		}
		return fmt.Errorf("unexpected type %q for option %q: %w", option.Type, option.Name, ErrDecodeOption)
	}

	return nil
}

var ErrCommandFormat = errors.New("invalid command format")

func Set(minGen int, maxGen int) Command {
	type options struct {
		LanguageOptions *struct {
			LocalizationCode string `option:"language"`
		} `option:"language"`
		GenerationOptions *struct {
			ID int `option:"generation_number"`
		} `option:"generation"`
	}

	minGenFloat := float64(minGen)

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
							MinValue:    &minGenFloat,
							MaxValue:    float64(maxGen),
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
							Choices: []*discordgo.ApplicationCommandOptionChoice{
								{
									Name:  "english",
									Value: "en",
								},
							},
						},
					},
				},
			},
		},
		handler: func(ctx context.Context, mdl *model.Model, sess *discordgo.Session, interaction *discordgo.InteractionCreate, opt options) error {
			if opt.LanguageOptions != nil {
				err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(opt.LanguageOptions.LocalizationCode))
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
			} else if opt.GenerationOptions != nil {
				err := mdl.SetGenerationByID(ctx, opt.GenerationOptions.ID)
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
	}
}
