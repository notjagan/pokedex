package command

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type handler[S any, T any] func(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate, S) (T, error)

type Command interface {
	ApplicationCommand() *discordgo.ApplicationCommand
	Handler(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
	Autocomplete(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
	Name() string
}

type command[T any] struct {
	applicationCommand *discordgo.ApplicationCommand
	handler            handler[*T, *discordgo.InteractionResponse]
	autocomplete       handler[*T, []*discordgo.ApplicationCommandOptionChoice]
}

func (cmd command[T]) ApplicationCommand() *discordgo.ApplicationCommand {
	return cmd.applicationCommand
}

func (cmd command[T]) Name() string {
	return cmd.applicationCommand.Name
}

func (cmd command[T]) Handler(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
) error {
	var structure T
	err := decodeOptions(interaction.ApplicationCommandData().Options, &structure)
	if err != nil {
		return fmt.Errorf("error while decoding options for command: %w", err)
	}

	resp, err := cmd.handler(ctx, mdl, sess, interaction, &structure)
	if err != nil {
		return fmt.Errorf("error while calling handler: %w", err)
	}

	sess.InteractionRespond(interaction.Interaction, resp)
	if err != nil {
		return fmt.Errorf("error while responding to command: %w", err)
	}

	return nil
}

func (cmd command[T]) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
) error {
	var structure T
	err := decodeOptions(interaction.ApplicationCommandData().Options, &structure)
	if err != nil {
		return fmt.Errorf("error while decoding options for autocomplete: %w", err)
	}

	choices, err := cmd.autocomplete(ctx, mdl, sess, interaction, &structure)
	if err != nil {
		return fmt.Errorf("error while calling autocompletion handler: %w", err)
	}

	sess.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{
			Choices: choices,
		},
	})
	if err != nil {
		return fmt.Errorf("error while sending autocompletions: %w", err)
	}

	return nil
}

var ErrDecodeOption = errors.New("error while decoding options")

type discordValue interface {
	string | int | bool
}

type discordField[T discordValue] struct {
	Value   T
	Focused bool
}

var fieldTypes = map[reflect.Type]bool{
	reflect.TypeOf(discordField[string]{}): true,
	reflect.TypeOf(discordField[int]{}):    true,
	reflect.TypeOf(discordField[bool]{}):   true,
}

func decodeOptions(options []*discordgo.ApplicationCommandInteractionDataOption, structure any) (ret error) {
	defer func() {
		r := recover()
		if err, ok := r.(reflect.ValueError); ok {
			ret = fmt.Errorf("reflection error while decoding options: %v", err.Error())
		} else if r != nil {
			panic(r)
		}
	}()

	value := reflect.Indirect(reflect.ValueOf(structure))
	if !value.CanAddr() {
		return fmt.Errorf("value is not addressable: %w", ErrDecodeOption)
	}

	m := make(map[string]reflect.Value, value.NumField())
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		tfield := value.Type().Field(i)
		option := tfield.Tag.Get("option")
		if option == "" {
			continue
		}

		if !field.CanSet() {
			return fmt.Errorf("field %q cannot be set: %w", tfield.Name, ErrDecodeOption)
		}
		m[option] = field
	}

	for _, option := range options {
		field, ok := m[option.Name]
		if !ok {
			return fmt.Errorf("unexpected option name %q: %w", option.Name, ErrDecodeOption)
		}

		if field.Kind() == reflect.Pointer {
			ptr := reflect.New(field.Type().Elem())
			field.Set(ptr)

			field = ptr.Elem()
		}
		if field.Kind() == reflect.Struct && fieldTypes[field.Type()] {
			backing := field.FieldByName("Value")
			backing.Set(reflect.Zero(backing.Type()))
			focused := field.FieldByName("Focused")
			focused.SetBool(option.Focused)

			field = backing
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
			if field.Kind() == reflect.Struct {
				err := decodeOptions(option.Options, field.Addr().Interface())
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
