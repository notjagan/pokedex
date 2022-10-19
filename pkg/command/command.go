package command

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type (
	Page struct {
		Limit  int
		Offset int
	}

	Command interface {
		ApplicationCommand() *discordgo.ApplicationCommand
		Handle(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
		Autocomplete(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
		Button(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate) error
		Name() string
	}

	buttonState interface {
		ActionName() byte
	}

	handler[S any, T any] func(context.Context, *model.Model, *discordgo.Session, *discordgo.InteractionCreate, S) (T, error)
	paginator[T any]      struct {
		Options T
		Page    Page
	}

	command[T any] struct {
		applicationCommand *discordgo.ApplicationCommand
		handle             handler[*T, *discordgo.InteractionResponseData]
		autocomplete       handler[*T, []*discordgo.ApplicationCommandOptionChoice]
		paginate           handler[paginator[T], *discordgo.InteractionResponseData]
		limit              *int
	}
)

func (paginator[T]) ActionName() byte {
	return 'p'
}

func customID(b buttonState) (string, error) {
	data, err := Marshal(b)
	if err != nil {
		return "", fmt.Errorf("failed to marshal button data: %w", err)
	}

	var uuid [4]byte
	rand.Reader.Read(uuid[:])

	return string(b.ActionName()) + data + string(uuid[:]), nil
}

func (cmd command[T]) ApplicationCommand() *discordgo.ApplicationCommand {
	return cmd.applicationCommand
}

func (cmd command[T]) Name() string {
	return cmd.applicationCommand.Name
}

var ErrUnrecognizedInteraction = errors.New("could not handle interaction")

func (cmd command[T]) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
) error {
	data := interaction.ApplicationCommandData()

	var structure T
	err := decodeOptions(data.Options, &structure)
	if err != nil {
		return fmt.Errorf("error while decoding options for command %q: %w", data.Name, err)
	}

	var body *discordgo.InteractionResponseData
	var typ discordgo.InteractionResponseType
	if cmd.handle != nil {
		body, err = cmd.handle(ctx, mdl, sess, interaction, &structure)
		if err != nil {
			return fmt.Errorf("error while calling handler: %w", err)
		}
		typ = discordgo.InteractionResponseChannelMessageWithSource
	} else if cmd.paginate != nil && cmd.limit != nil {
		paginator := paginator[T]{
			Options: structure,
			Page: Page{
				Limit:  *cmd.limit,
				Offset: 0,
			},
		}
		body, err = cmd.paginate(ctx, mdl, sess, interaction, paginator)
		if err != nil {
			return fmt.Errorf("error while calling handler: %w", err)
		}
		typ = discordgo.InteractionResponseChannelMessageWithSource
	} else {
		return fmt.Errorf("handler not found for command %q: %w", data.Name, ErrUnrecognizedInteraction)
	}

	err = sess.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
		Type: typ,
		Data: body,
	})
	if err != nil {
		return fmt.Errorf("error while responding to command: %w", err)
	}

	return nil
}

func (cmd command[T]) Button(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
) error {
	data := interaction.MessageComponentData()
	action := data.CustomID[0]
	id := data.CustomID[1:]

	switch action {
	case paginator[T]{}.ActionName():
		p, err := Unmarshal[paginator[T]](id)
		if err != nil {
			return fmt.Errorf("error while deserializing pagination data: %w", err)
		}

		body, err := cmd.paginate(ctx, mdl, sess, interaction, *p)
		if err != nil {
			return fmt.Errorf("error while calling pagination handler: %w", err)
		}

		edit := discordgo.NewMessageEdit(interaction.ChannelID, interaction.Message.ID)
		edit.Content = &body.Content
		edit.Embeds = body.Embeds
		edit.Components = body.Components
		_, err = sess.ChannelMessageEditComplex(edit)
		if err != nil {
			return fmt.Errorf("failed to edit message: %w", err)
		}

		err = sess.InteractionRespond(interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseUpdateMessage,
		})
		if err != nil {
			return fmt.Errorf("failed to complete interaction: %w", err)
		}

		return nil
	default:
		return fmt.Errorf("unknown button action %q: %w", action, ErrUnrecognizedInteraction)
	}
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

var ErrEncodeOptions = errors.New("error while encoding options")

type encoder struct {
	Writer io.Writer
}

func (e *encoder) encode(structure any) error {
	value := reflect.ValueOf(structure)
	switch value.Kind() {
	case reflect.Int:
		err := binary.Write(e.Writer, binary.BigEndian, int32(value.Int()))
		if err != nil {
			return fmt.Errorf("failed to write int value: %w", err)
		}
	case reflect.Bool:
		err := binary.Write(e.Writer, binary.BigEndian, value.Bool())
		if err != nil {
			return fmt.Errorf("failed to write boolean value: %w", err)
		}
	case reflect.String:
		b := []byte(value.String())
		err := binary.Write(e.Writer, binary.BigEndian, uint8(len(b)))
		if err != nil {
			return fmt.Errorf("failed to write length for string value: %w", err)
		}

		_, err = e.Writer.Write(b)
		if err != nil {
			return fmt.Errorf("failed to write string value: %w", err)
		}
	case reflect.Pointer:
		if value.IsNil() {
			err := binary.Write(e.Writer, binary.BigEndian, false)
			if err != nil {
				return fmt.Errorf("failed to write nil marker for pointer: %w", err)
			}
		} else {
			err := binary.Write(e.Writer, binary.BigEndian, true)
			if err != nil {
				return fmt.Errorf("failed to write non-nil marker for pointer: %w", err)
			}

			err = e.encode(value.Elem().Interface())
			if err != nil {
				return fmt.Errorf("error while encoding element for pointer: %w", err)
			}
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			err := e.encode(field.Interface())
			if err != nil {
				return fmt.Errorf("error while encoding field for struct: %w", err)
			}
		}
	default:
		return fmt.Errorf("unsupported type in options: %w", ErrEncodeOptions)
	}

	return nil
}

func Marshal(structure any) (string, error) {
	var buf bytes.Buffer
	enc := encoder{&buf}
	err := enc.encode(structure)
	if err != nil {
		return "", fmt.Errorf("failed to marshall structure: %w", err)
	}

	return buf.String(), nil
}

type decoder struct {
	Reader io.Reader
}

func (d *decoder) decodeValue(value reflect.Value) error {
	if !value.CanSet() {
		return fmt.Errorf("cannot set fields for value of type %q: %w", value.Type().String(), ErrDecodeOption)
	}

	switch value.Kind() {
	case reflect.Int:
		var v int32
		err := binary.Read(d.Reader, binary.BigEndian, &v)
		if err != nil {
			return fmt.Errorf("failed to read int value: %w", err)
		}

		value.SetInt(int64(v))
	case reflect.Bool:
		var v bool
		err := binary.Read(d.Reader, binary.BigEndian, &v)
		if err != nil {
			return fmt.Errorf("failed to read boolean value: %w", err)
		}

		value.SetBool(v)
	case reflect.String:
		var l uint8
		err := binary.Read(d.Reader, binary.BigEndian, &l)
		if err != nil {
			return fmt.Errorf("failed to read length for string value: %w", err)
		}

		buf := make([]byte, l)
		_, err = io.ReadFull(d.Reader, buf)
		if err != nil {
			return fmt.Errorf("failed to read string value: %w", err)
		}

		value.SetString(string(buf))
	case reflect.Pointer:
		var f bool
		err := binary.Read(d.Reader, binary.BigEndian, &f)
		if err != nil {
			return fmt.Errorf("failed to check if pointer is nil: %w", err)
		}

		if f {
			ptr := reflect.New(value.Type().Elem())
			value.Set(ptr)
			err := d.decodeValue(ptr.Elem())
			if err != nil {
				return fmt.Errorf("error while decoding options for pointer element: %w", err)
			}
		} else {
			value.Set(reflect.Zero(value.Type()))
		}
	case reflect.Struct:
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			err := d.decodeValue(field)
			if err != nil {
				return fmt.Errorf("error while decoding options for struct field: %w", err)
			}
		}
	default:
		return fmt.Errorf("unsupported type in options: %w", ErrDecodeOption)
	}

	return nil
}

func (d *decoder) decode(pointer any) error {
	value := reflect.ValueOf(pointer)
	if value.Kind() != reflect.Pointer && value.Type().Elem().Kind() != reflect.Struct {
		return fmt.Errorf("attempted decode into non-pointer field: %w", ErrDecodeOption)
	}

	return d.decodeValue(value.Elem())
}

func Unmarshal[T any](data string) (*T, error) {
	var structure T
	dec := decoder{
		Reader: bytes.NewBuffer([]byte(data)),
	}
	err := dec.decode(&structure)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return &structure, nil
}
