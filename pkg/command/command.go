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

type command[T any] struct {
	applicationCommand *discordgo.ApplicationCommand
	handler            func(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate, T) error
}

func (cmd command[T]) ApplicationCommand() *discordgo.ApplicationCommand {
	return cmd.applicationCommand
}

func (cmd command[T]) Name() string {
	return cmd.applicationCommand.Name
}

func (cmd command[T]) CallHandler(ctx context.Context, mdl *model.Model, sess *discordgo.Session, interaction *discordgo.InteractionCreate) error {
	var structure T
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

func decodeOptions(options []*discordgo.ApplicationCommandInteractionDataOption, pointer any) (ret error) {
	defer func() {
		r := recover()
		if err, ok := r.(reflect.ValueError); ok {
			ret = fmt.Errorf("reflection error while decoding options: %v", err.Error())
		} else if r != nil {
			panic(r)
		}
	}()

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
