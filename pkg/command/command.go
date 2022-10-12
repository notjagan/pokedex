package command

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type Command interface {
	ApplicationCommand() *discordgo.ApplicationCommand
	CallHandler(*model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
	Name() string
}

type command[S any] struct {
	applicationCommand *discordgo.ApplicationCommand
	handler            func(*model.Model, *discordgo.Session, *discordgo.InteractionCreate, S) error
}

func (cmd command[S]) ApplicationCommand() *discordgo.ApplicationCommand {
	return cmd.applicationCommand
}

func (cmd command[S]) Name() string {
	return cmd.applicationCommand.Name
}

func (cmd command[S]) CallHandler(mdl *model.Model, sess *discordgo.Session, interaction *discordgo.InteractionCreate) error {
	var structure S
	err := decodeOptions(interaction.ApplicationCommandData().Options, &structure)
	if err != nil {
		return fmt.Errorf("error while decoding options for command: %w", err)
	}
	cmd.handler(mdl, sess, interaction, structure)

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

	for _, S := range options {
		field, ok := m[S.Name]
		if !ok {
			return fmt.Errorf("unexpected option name %q: %w", S.Name, ErrDecodeOption)
		}

		switch S.Type {
		case discordgo.ApplicationCommandOptionString:
			if field.Kind() == reflect.String {
				field.SetString(S.StringValue())
				continue
			}
		case discordgo.ApplicationCommandOptionInteger:
			if field.Kind() == reflect.Int {
				field.SetInt(S.IntValue())
				continue
			}
		case discordgo.ApplicationCommandOptionBoolean:
			if field.Kind() == reflect.Bool {
				field.SetBool(S.BoolValue())
				continue
			}
		default:
			return fmt.Errorf("unsupported type %q for option %q: %w", S.Type, S.Name, ErrDecodeOption)
		}
		return fmt.Errorf("unexpected type %q for option %q: %w", S.Type, S.Name, ErrDecodeOption)
	}

	return nil
}
