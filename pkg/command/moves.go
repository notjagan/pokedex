package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type movesOptions struct {
	PokemonName discordField[string] `option:"pokemon"`
	Level       int                  `option:"level"`
}

type movesResponder struct {
	queryLimit        int
	autocompleteLimit int
	moveCount         int
	learnMethodNames  []model.LearnMethodName
	emojis            Emojis
}

func (resp movesResponder) Paginate(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	p paginator[movesOptions],
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

	methods, err := mdl.LearnMethodsByName(ctx, resp.learnMethodNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get learn methods: %w", err)
	}

	pms, hasNext, err := pokemon.SearchPokemonMoves(ctx, methods, &p.Options.Level, &resp.moveCount, p.Page.Limit, p.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("could not get moves for pokemon %q: %w", pokemon.Name, err)
	}

	fields, err := movesToFields(ctx, pms, resp.emojis)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pokemon moves to discord fields: %w", err)
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s, %s", pokemonName, genName),
		Description: fmt.Sprintf("Lv. %d", p.Options.Level),
		Fields:      fields,
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

func (resp movesResponder) Initial() Page {
	return Page{
		Offset: 0,
		Limit:  resp.queryLimit,
	}
}

func (resp movesResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *movesOptions,
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

func (builder *Builder) moves(ctx context.Context) (Command, error) {
	minLevel := float64(builder.metadata.MinLevel)
	maxLevel := float64(builder.metadata.MaxLevel)

	resp := movesResponder{
		queryLimit:        builder.config.MoveLimit,
		autocompleteLimit: builder.config.AutocompleteLimit,
		moveCount:         builder.metadata.MoveCount,
		learnMethodNames: []model.LearnMethodName{
			model.LevelUp,
		},
		emojis: builder.emojis,
	}

	return command[movesOptions]{
		pager:         resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
			Name:        "moves",
			Description: "Most likely moveset for a Pokemon at a given level.",
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
					Name:        "level",
					Description: "Level of the Pokemon",
					Required:    true,
					MinValue:    &minLevel,
					MaxValue:    maxLevel,
				},
			},
		},
	}, nil
}
