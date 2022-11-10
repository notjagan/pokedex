package command

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type dexOptions struct {
	Pokemon *struct {
		Name discordField[string] `option:"pokemon"`
	} `option:"pokemon"`
}

type dexResponder struct {
	autocompleteLimit int
	emojis            Emojis
}

func (resp dexResponder) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *dexOptions,
) (*discordgo.InteractionResponseData, error) {
	pokemon, err := mdl.PokemonByName(ctx, opt.Pokemon.Name.Value)
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

	titleStrings := make([]string, 0, 3)

	name, err := pokemon.LocalizedName(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting localized name for pokemon: %w", err)
	}
	titleStrings = append(titleStrings, name)

	combo, err := pokemon.TypeCombo(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get type combo for pokemon: %w", err)
	}

	t1, err := resp.emojis.Emoji(combo.Type1.Name)
	if err != nil {
		return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
	}
	titleStrings = append(titleStrings, t1)

	if combo.Type2 != nil {
		t2, err := resp.emojis.Emoji(combo.Type2.Name)
		if err != nil {
			return nil, fmt.Errorf("error while constructing first type emoji string: %w", err)
		}
		titleStrings = append(titleStrings, t2)
	}

	gen, err := mdl.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting generation for model version: %w", err)
	}
	genName, err := gen.LocalizedName(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting localized name for model generation: %w", err)
	}

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

	fields := make([]*discordgo.MessageEmbedField, 0, 8)

	abilities, err := pokemon.Abilities(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting abilities for pokemon: %w", err)
	}

	visibleAbilities := make([]string, 0, len(abilities))
	hiddenAbilities := make([]string, 0, len(abilities))
	for _, ability := range abilities {
		name, err := ability.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting localized name for ability: %w", err)
		}

		if ability.IsHidden {
			hiddenAbilities = append(hiddenAbilities, name)
		} else {
			visibleAbilities = append(visibleAbilities, name)
		}
	}

	visibleAbilityField := discordgo.MessageEmbedField{Name: "Abilities", Inline: true}
	hiddenAbilityField := discordgo.MessageEmbedField{Name: "Hidden Abilities", Inline: true}
	if len(visibleAbilities) > 0 {
		visibleAbilityField.Value = strings.Join(visibleAbilities, ", ")
		fields = append(fields, &visibleAbilityField)
	} else if len(hiddenAbilities) == 0 {
		visibleAbilityField.Value = "_None_"
		fields = append(fields, &visibleAbilityField)
	}

	if len(hiddenAbilities) > 0 {
		hiddenAbilityField.Value = strings.Join(hiddenAbilities, ", ")
		fields = append(fields, &hiddenAbilityField)
	}

	padding := 3 - len(fields)
	for i := 0; i < padding; i++ {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "\u200b",
			Value:  "\u200b",
			Inline: true,
		})
	}

	is, err := mdl.IntrinsicStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting all intrinsic stats: %w", err)
	}

	for _, stat := range is {
		bs, err := pokemon.BaseStat(ctx, stat)
		if err != nil {
			return nil, fmt.Errorf("error while getting base stat for pokemon: %w", err)
		}

		name, err := stat.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("error while getting localized name for stat: %w", err)
		}

		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   name,
			Value:  strconv.Itoa(bs),
			Inline: true,
		})
	}

	return &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       strings.Join(titleStrings, " "),
				Description: genName,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: "attachment://sprite.png",
				},
				Fields: fields,
			},
		},
		Files: []*discordgo.File{
			{
				Name:        "sprite.png",
				ContentType: "image/png",
				Reader:      reader,
			},
		},
	}, nil
}

func (resp dexResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *dexOptions,
) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch {
	case opt.Pokemon != nil:
		if opt.Pokemon.Name.Focused {
			s := pokemonSearcher{
				model:  mdl,
				prefix: opt.Pokemon.Name.Value,
				limit:  resp.autocompleteLimit,
			}
			return searchChoices[*model.Pokemon](ctx, s)
		}
	default:
		return nil, fmt.Errorf("no recognized subcommand in focus: %w", ErrCommandFormat)
	}

	return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
}

func (builder *Builder) dex(ctx context.Context) (Command, error) {
	resp := dexResponder{
		autocompleteLimit: builder.config.AutocompleteLimit,
		emojis:            builder.emojis,
	}

	return command[dexOptions]{
		handler:       resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
			Name:        "dex",
			Description: "Fetch game data for a specified resource.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionSubCommand,
					Name:        "pokemon",
					Description: "Fetch data for a Pokemon",
					Options: []*discordgo.ApplicationCommandOption{
						{
							Type:         discordgo.ApplicationCommandOptionString,
							Name:         "pokemon",
							Description:  "Name of the Pokemon",
							Required:     true,
							Autocomplete: true,
						},
					},
				},
			},
		},
	}, nil
}
