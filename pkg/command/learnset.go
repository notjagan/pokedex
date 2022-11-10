package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type learnsetOptions struct {
	PokemonName discordField[string] `option:"pokemon"`
	MaxLevel    *int                 `option:"max_level"`
	EggMoves    *bool                `option:"egg_moves"`
}

type learnsetResponder struct {
	queryLimit        int
	autocompleteLimit int
	learnMethodNames  []model.LearnMethodName
	emojis            Emojis
}

func (resp learnsetResponder) Paginate(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	p paginator[learnsetOptions],
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

	methodNames := make([]model.LearnMethodName, len(resp.learnMethodNames), 2)
	copy(methodNames, resp.learnMethodNames)
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
	fields, err := movesToFields(ctx, pms, resp.emojis)
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
}

func (resp learnsetResponder) Initial() Page {
	return Page{
		Offset: 0,
		Limit:  resp.queryLimit,
	}
}

func (resp learnsetResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *learnsetOptions,
) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch {
	case opt.PokemonName.Focused:
		s := pokemonSearcher{
			model:  mdl,
			prefix: opt.PokemonName.Value,
			limit:  resp.autocompleteLimit,
		}
		return searchChoices[*model.Pokemon](ctx, s)
	default:
		return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
	}
}

func (builder *Builder) learnset(ctx context.Context) (Command, error) {
	minLevel := float64(builder.metadata.MinLevel)
	maxLevel := float64(builder.metadata.MaxLevel)

	resp := learnsetResponder{
		queryLimit:        builder.config.MoveLimit,
		autocompleteLimit: builder.config.AutocompleteLimit,
		learnMethodNames: []model.LearnMethodName{
			model.LevelUp,
		},
		emojis: builder.emojis,
	}

	return command[learnsetOptions]{
		pager:         resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
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
	}, nil
}
