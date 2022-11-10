package bot

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/notjagan/pokedex/pkg/command"
	"github.com/notjagan/pokedex/pkg/config"
	"github.com/notjagan/pokedex/pkg/model"
)

type Bot struct {
	config   config.Config
	session  *discordgo.Session
	commands map[string]command.Command
	models   map[string]*model.Model
	emojis   command.Emojis
}

func New(ctx context.Context, config config.Config) (*Bot, error) {
	sess, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate discord bot: %w", err)
	}

	emojis := make(command.Emojis)
	cmds, err := command.All(ctx, config, emojis)
	if err != nil {
		return nil, fmt.Errorf("error while getting all commands for bot: %w", err)
	}

	return &Bot{
		session:  sess,
		config:   config,
		commands: cmds,
		models:   make(map[string]*model.Model),
		emojis:   emojis,
	}, nil
}

func (bot *Bot) Close() {
	log.Println("Shutting down.")
	for _, model := range bot.models {
		err := model.Close()
		if err != nil {
			log.Printf("error while closing model: %v", err)
		}
	}
	err := bot.session.Close()
	if err != nil {
		log.Printf("error while closing discord session: %v", err)
	}
}

func (bot *Bot) addModel(ctx context.Context, ID string, locale discordgo.Locale) (*model.Model, error) {
	mdl, err := model.New(ctx, bot.config.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while instantiating model: %w", err)
	}
	bot.models[ID] = mdl

	err = mdl.SetLanguageByLocale(ctx, locale)
	if err != nil {
		return nil, fmt.Errorf("error while setting language: %w", err)
	}

	err = mdl.SetVersionByName(ctx, string(model.VersionNameSword))
	if err != nil {
		return nil, fmt.Errorf("error while setting default version: %w", err)
	}

	return mdl, nil
}

var ErrNoMatchingModel = errors.New("no matching model")

func (bot *Bot) initialize(ctx context.Context) error {
	err := bot.session.Open()
	if err != nil {
		return fmt.Errorf("failed to start discord session: %w", err)
	}

	connected := make(chan error)

	bot.session.AddHandler(func(_ *discordgo.Session, create *discordgo.GuildCreate) {
		_, err := bot.addModel(ctx, create.Guild.ID, discordgo.Locale(create.PreferredLocale))
		if err != nil {
			log.Printf("failed to add guild %q: %v", create.Guild.Name, err)
			return
		}

		if create.Guild.ID == bot.config.Discord.CommandConfig.ResourceGuildID {
			connected <- err
			for _, emoji := range create.Guild.Emojis {
				bot.emojis[emoji.Name] = emoji
			}
		}
	})

	select {
	case err := <-connected:
		if err != nil {
			return fmt.Errorf("failed to connect to resource guild: %w", err)
		}
	case <-time.After(time.Duration(bot.config.Discord.CommandConfig.ResourceTimeout) * time.Millisecond):
		return fmt.Errorf("timeout while connecting to resource server")
	}

	err = bot.registerCommands(ctx)
	if err != nil {
		return fmt.Errorf("error while registering commands: %w", err)
	}

	err = bot.unregisterRemovedCommands(ctx)
	if err != nil {
		return fmt.Errorf("error while unregistering removed commands: %w", err)
	}

	return nil
}

func (bot *Bot) Run(ctx context.Context) error {
	err := bot.initialize(ctx)
	if err != nil {
		return fmt.Errorf("error while initializing bot: %w", err)
	}

	log.Println("Hosting Pokedex bot.")
	defer bot.Close()
	<-ctx.Done()

	return nil
}

func (bot *Bot) registerCommands(ctx context.Context) error {
	bot.session.AddHandler(func(sess *discordgo.Session, interaction *discordgo.InteractionCreate) {
		var mdl *model.Model
		switch {
		case interaction.Member != nil:
			guild, err := sess.State.Guild(interaction.GuildID)
			if err != nil {
				log.Printf("could not find guild while handling interaction: %v", err)
				return
			}
			var ok bool
			mdl, ok = bot.models[guild.ID]
			if !ok {
				log.Printf("no model found for guild %q while handling interaction: %v", guild.Name, ErrNoMatchingModel)
				return
			}
		case interaction.User != nil:
			user := interaction.User
			var ok bool
			mdl, ok = bot.models[user.ID]
			if !ok {
				var err error
				mdl, err = bot.addModel(ctx, user.ID, discordgo.Locale(user.Locale))
				if err != nil {
					log.Printf("failed to create model for user %q: %v", user.Username, err)
					return
				}
			}
		default:
			log.Printf("failed to find user associated with interaction")
			return
		}

		switch interaction.Type {
		case discordgo.InteractionApplicationCommand, discordgo.InteractionApplicationCommandAutocomplete:
			data := interaction.ApplicationCommandData()
			cmd, ok := bot.commands[data.Name]
			if !ok {
				log.Printf("unrecognized command %q", data.Name)
				return
			}

			switch interaction.Type {
			case discordgo.InteractionApplicationCommand:
				log.Printf("Handling command %q.", cmd.Name())
				err := cmd.Handle(ctx, mdl, sess, interaction)
				if err != nil {
					log.Printf("error while executing command %q: %v", cmd.Name(), err)
				}
				return
			case discordgo.InteractionApplicationCommandAutocomplete:
				err := cmd.Autocomplete(ctx, mdl, sess, interaction)
				if err != nil {
					log.Printf("error while generating autocompletions for command %q: %v", cmd.Name(), err)
				}
				return
			default:
				log.Printf("unrecognized interaction type %s for command %q", interaction.Type.String(), cmd.Name())
			}
		case discordgo.InteractionMessageComponent:
			data := interaction.MessageComponentData()
			switch data.ComponentType {
			case discordgo.ButtonComponent:
				reader := bytes.NewReader([]byte(data.CustomID))
				followUp, err := command.ButtonFollowUp(reader)
				if err != nil {
					log.Printf("could not read follow-up command: %v", err)
					return
				}

				var name string
				if followUp != nil {
					name = *followUp
				} else {
					name = interaction.Message.Interaction.Name
				}
				cmd, ok := bot.commands[name]
				if !ok {
					log.Printf("unrecognized command %q", name)
					return
				}

				err = cmd.Button(ctx, mdl, sess, interaction, reader)
				if err != nil {
					log.Printf("error while handling button press for command %q: %v", cmd.Name(), err)
				}
				return

			default:
				log.Println("unrecognized component type for message interaction")
			}
		default:
			log.Printf("unrecognized interaction type %s", interaction.Type.String())
		}
	})

	cmds := make([]*discordgo.ApplicationCommand, len(bot.commands))
	i := 0
	for _, cmd := range bot.commands {
		ac := cmd.ApplicationCommand()
		cmds[i] = &ac
		i++
	}

	_, err := bot.session.ApplicationCommandBulkOverwrite(bot.session.State.User.ID, "", cmds)
	if err != nil {
		return fmt.Errorf("failed to create commands: %w", err)
	}

	return nil
}

func (bot *Bot) unregisterRemovedCommands(ctx context.Context) error {
	appID := bot.session.State.User.ID

	cmds, err := bot.session.ApplicationCommands(appID, "")
	if err != nil {
		return fmt.Errorf("failed to get registered commands: %w", err)
	}

	for _, cmd := range cmds {
		if _, ok := bot.commands[cmd.Name]; !ok {
			err := bot.session.ApplicationCommandDelete(appID, "", cmd.ID)
			if err != nil {
				return fmt.Errorf("failed to delete command %q: %w", cmd.Name, err)
			}
		}
	}

	return nil
}
