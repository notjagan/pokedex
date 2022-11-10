package command

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/model"
)

type versionOptions struct {
	Name *discordField[string] `option:"version"`
}

type versionResponder struct {
	autocompleteLimit int
}

func (resp versionResponder) Handle(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *versionOptions,
) (*discordgo.InteractionResponseData, error) {
	if opt.Name == nil {
		name, err := mdl.Version.LocalizedName(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not localize current version name: %w", err)
		}

		return &discordgo.InteractionResponseData{
			Content: fmt.Sprintf("Currently using Pokemon %s.", name),
		}, nil
	} else {
		err := mdl.SetVersionByName(ctx, opt.Name.Value)
		if err != nil {
			return nil, fmt.Errorf("error while changing version: %w", err)
		}

		return &discordgo.InteractionResponseData{
			Content: "Version successfully changed.",
		}, nil
	}
}

func (resp versionResponder) Autocomplete(
	ctx context.Context,
	mdl *model.Model,
	sess *discordgo.Session,
	interaction *discordgo.InteractionCreate,
	opt *versionOptions,
) ([]*discordgo.ApplicationCommandOptionChoice, error) {
	switch {
	case opt.Name != nil && opt.Name.Focused:
		s := versionSearcher{
			model:  mdl,
			prefix: opt.Name.Value,
			limit:  resp.autocompleteLimit,
		}
		return searchChoices[*model.Version](ctx, s)
	default:
		return nil, fmt.Errorf("no recognized field in focus: %w", ErrCommandFormat)
	}
}

func (builder *Builder) version(ctx context.Context) (Command, error) {
	resp := versionResponder{
		autocompleteLimit: builder.config.AutocompleteLimit,
	}

	return command[versionOptions]{
		handler:       resp,
		autocompleter: resp,
		command: discordgo.ApplicationCommand{
			Name:        "version",
			Description: "Get/set the current Pokedex game version.",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:         discordgo.ApplicationCommandOptionString,
					Name:         "version",
					Description:  "Game version to pull data from",
					Required:     false,
					Autocomplete: true,
				},
			},
		},
	}, nil
}
