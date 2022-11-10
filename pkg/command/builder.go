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

type Emojis map[string]*discordgo.Emoji

type Builder struct {
	model *model.Model

	config   config.CommandConfig
	metadata config.PokemonMetadata
	funcs    []commandFunc
	emojis   Emojis
}

func NewBuilder(ctx context.Context, mdl *model.Model, cfg config.Config, emojis Emojis) *Builder {
	mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCodeEnglish)
	return &Builder{
		model:    mdl,
		config:   cfg.Discord.CommandConfig,
		metadata: cfg.Pokemon.Metadata,
		funcs: []commandFunc{
			(*Builder).language,
			(*Builder).version,
			(*Builder).learnset,
			(*Builder).moves,
			(*Builder).weak,
			(*Builder).coverage,
			(*Builder).dex,
		},
		emojis: emojis,
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

func (emojis Emojis) Emoji(name string) (string, error) {
	emoji1, ok := emojis[name+"1"]
	if !ok {
		return "", fmt.Errorf("could not find first emoji for resource %q: %w", name, ErrNoEmoji)
	}

	emoji2, ok := emojis[name+"2"]
	if !ok {
		return "", fmt.Errorf("could not find second emoji for resource %q: %w", name, ErrNoEmoji)
	}

	return fmt.Sprintf("<:%v:%v><:%v:%v>", emoji1.Name, emoji1.ID, emoji2.Name, emoji2.ID), nil
}

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
	homeID, err := customID(phome, nil)
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
	prevID, err := customID(pprev, nil)
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
	nextID, err := customID(pnext, nil)
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

func All(ctx context.Context, cfg config.Config, emojis Emojis) (map[string]Command, error) {
	mdl, err := model.New(ctx, cfg.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	builder := NewBuilder(ctx, mdl, cfg, emojis)
	defer builder.Close(ctx)

	return builder.all(ctx)
}
