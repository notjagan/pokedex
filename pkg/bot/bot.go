package bot

import (
	"context"
	"errors"
	"fmt"
	"log"

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
}

func New(ctx context.Context, config config.Config) (*Bot, error) {
	sess, err := discordgo.New("Bot " + config.Discord.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to instantiate discord bot: %w", err)
	}

	cmds, err := command.All(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("error while getting all commands for bot: %w", err)
	}

	return &Bot{
		session:  sess,
		config:   config,
		commands: cmds,
		models:   make(map[string]*model.Model),
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

func (bot *Bot) addGuild(ctx context.Context, guild *discordgo.Guild) error {
	mdl, err := model.New(ctx, bot.config.DB.Path)
	if err != nil {
		return fmt.Errorf("error while instantiating model for guild %q: %w", guild.Name, err)
	}
	bot.models[guild.ID] = mdl

	err = mdl.SetLanguageByLocale(ctx, discordgo.Locale(guild.PreferredLocale))
	if err != nil {
		return fmt.Errorf("error while setting language: %w", err)
	}

	gen, err := mdl.LatestGeneration(ctx)
	if err != nil {
		return fmt.Errorf("error while getting default generation: %w", err)
	}
	mdl.Generation = gen

	return nil
}

func (bot *Bot) removeGuild(guild *discordgo.Guild) {
	delete(bot.models, guild.ID)
}

var ErrNoMatchingModel = errors.New("no matching model")

func (bot *Bot) model(guild *discordgo.Guild) (*model.Model, error) {
	model, ok := bot.models[guild.ID]
	if !ok {
		return nil, fmt.Errorf("could not find model for guild %q: %w", guild.Name, ErrNoMatchingModel)
	}

	return model, nil
}

func (bot *Bot) initialize(ctx context.Context) error {
	err := bot.session.Open()
	if err != nil {
		return fmt.Errorf("failed to start discord session: %w", err)
	}

	bot.session.AddHandler(func(_ *discordgo.Session, create *discordgo.GuildCreate) {
		err := bot.addGuild(ctx, create.Guild)
		if err != nil {
			log.Printf("failed to add guild %q: %v", create.Guild.Name, err)
		}
	})
	bot.session.AddHandler(func(_ *discordgo.Session, delete *discordgo.GuildDelete) {
		bot.removeGuild(delete.Guild)
		if err != nil {
			log.Printf("failed to add guild %q: %v", delete.Guild.Name, err)
		}
	})

	err = bot.registerCommands(ctx)
	if err != nil {
		return fmt.Errorf("error while registering commands: %w", err)
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
		guild, err := sess.State.Guild(interaction.GuildID)
		if err != nil {
			log.Printf("could not find guild while handling interaction: %v", err)
			return
		}
		mdl, err := bot.model(guild)
		if err != nil {
			log.Printf("no model found for guild while handling interaction: %v", err)
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
				log.Printf("COMMAND %q in GUILD %q", cmd.Name(), guild.Name)
				err = cmd.Handle(ctx, mdl, sess, interaction)
				if err != nil {
					log.Printf("error while executing command %q: %v", cmd.Name(), err)
				}
				return
			case discordgo.InteractionApplicationCommandAutocomplete:
				err = cmd.Autocomplete(ctx, mdl, sess, interaction)
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
				name := interaction.Message.Interaction.Name
				cmd, ok := bot.commands[name]
				if !ok {
					log.Printf("unrecognized command %q", name)
					return
				}

				err = cmd.Button(ctx, mdl, sess, interaction)
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

	for _, cmd := range bot.commands {
		_, err := bot.session.ApplicationCommandCreate(bot.session.State.User.ID, "", cmd.ApplicationCommand())
		if err != nil {
			return fmt.Errorf("failed to create command %q: %w", cmd.Name(), err)
		}
	}

	return nil
}
