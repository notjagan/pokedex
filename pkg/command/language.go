package command

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type languageOptions struct {
	LocalizationCode *string `option:"language"`
}

type languageResponder struct{}

func (resp languageResponder) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *languageOptions,
) (*discordgo.InteractionResponseData, error) {
	if opt.LocalizationCode == nil {
		name, err := mdl.Language.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not localize current language name: %w", err)
		}

		return &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Language is currently %q.", name),
		}, nil
	} else {
		err := mdl.SetLanguageByLocalizationCode(ctx, model.LocalizationCode(*opt.LocalizationCode))
		if err != nil {
			return nil, fmt.Errorf("error while changing language: %w", err)
		}

		return &discordgo.InteractionResponseData{
			Content: "Language successfully changed.",
		}, nil
	}
}

func (builder *Builder) language(ctx context.Context) (Command, error) {
	s := languageSearcher{model: builder.model}
	langChoices, err := searchChoices[*model.Language](ctx, s)
	if err != nil {
		return nil, fmt.Errorf("could not get available language choices: %w", err)
	}

	return command[languageOptions]{
		handler: languageResponder{},
		command: discordgo.ApplicationCommand{
			Name:        "language",
			Description: "Get/set the the current Pokedex language.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "language",
					Description: "Language to set Pokedex to",
					Required:    false,
					Choices:     langChoices,
				},
			},
		},
	}, nil
}
