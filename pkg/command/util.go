package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

var ErrCommandFormat = errors.New("invalid command format")

var ErrMissingResourceGuild = errors.New("resource guild not found")

func movesToFields(ctx context.Context, pms []model.PokemonMove, emojis Emojis) ([]*discordgo.MessageEmbedField, error) {
	fields := make([]*discordgo.MessageEmbedField, len(pms))
	for i, move := range pms {
		values := make([]string, 0, 5)

		name, err := move.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get localized name for move %q: %w", move.Name, err)
		}

		typ, err := move.Type(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting type for move %q: %w", move.Name, err)
		}
		if !typ.IsUnknown() {
			typeString, err := emojis.Emoji(typ.Name)
			if err != nil {
				return nil, fmt.Errorf("error while constructing type emoji string for move %q: %w", move.Name, err)
			}
			values = append(values, typeString)
		}

		class, err := move.DamageClass(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting damage class for move %q: %w", move.Name, err)
		}
		classString, err := emojis.Emoji(class.Name)
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
			Name:  fmt.Sprintf("Lv. %-2d ▸ %s", move.Level, name),
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

func (p paginator[T]) moveButtons(hasNext bool, cmds commands) (*discordgo.ActionsRow, error) {
	cmd, err := optionCommand[T](cmds)
	if err != nil {
		return nil, fmt.Errorf("could not find command in registry: %w", err)
	}

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
	homeID, err := customID(phome, cmd.Name())
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
	prevID, err := customID(pprev, cmd.Name())
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
	nextID, err := customID(pnext, cmd.Name())
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

type efficacyNames struct {
	doubleStrong string
	strong       string
	neutral      string
	weak         string
	doubleWeak   string
	immune       string
}

func efficaciesToFields(
	ctx context.Context,
	effs []model.TypeEfficacy,
	includeAll bool,
	names efficacyNames,
	emojis Emojis,
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
		emoji, err := emojis.Emoji(typ.Name)
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

func pokemonSpriteFile(ctx context.Context, pokemon *model.Pokemon) (*discordgo.File, error) {
	sprites, err := pokemon.Sprites(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting sprites for pokemon: %w", err)
	}

	sprite := sprites.Front.Default
	spritePath, err := sprite.Filepath()
	if err != nil {
		return nil, fmt.Errorf("could not get filepath for pokemon sprite: %w", err)
	}

	reader, err := os.Open(string(spritePath))
	if err != nil {
		return nil, fmt.Errorf("could not open reader for sprite path %q: %w", spritePath, err)
	}

	return &discordgo.File{
		Name:        fmt.Sprintf("%s.png", pokemon.Name),
		ContentType: "image/png",
		Reader:      reader,
	}, nil
}
