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
	commands []command.Command
	models   map[string]*model.Model
}

func New(ctx context.Context, config config.Config) (*Bot, error) {
	cmds, err := command.All(ctx, config.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while getting all commands for bot: %w", err)
	}

	return &Bot{
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
	mdl.SetGeneration(ctx, gen)

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
	sess, err := discordgo.New("Bot " + bot.config.Discord.Token)
	if err != nil {
		return fmt.Errorf("failed to instantiate discord bot: %w", err)
	}
	bot.session = sess

	err = bot.session.Open()
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

func (bot *Bot) register(ctx context.Context, cmd command.Command) error {
	_, err := bot.session.ApplicationCommandCreate(bot.session.State.User.ID, "", cmd.ApplicationCommand())
	if err != nil {
		return fmt.Errorf("failed to create command %q: %w", cmd.Name(), err)
	}
	bot.session.AddHandler(func(sess *discordgo.Session, interaction *discordgo.InteractionCreate) {
		if interaction.ApplicationCommandData().Name == cmd.Name() {
			guild, err := sess.State.Guild(interaction.GuildID)
			if err != nil {
				log.Printf("could not find guild while executing command %q: %v", cmd.Name(), err)
				return
			}

			mdl, err := bot.model(guild)
			if err != nil {
				log.Printf("no model found for guild while executing command %q: %v", cmd.Name(), err)
				return
			}

			log.Printf("COMMAND %q in GUILD %q", cmd.Name(), guild.Name)
			err = cmd.CallHandler(ctx, mdl, sess, interaction)
			if err != nil {
				log.Printf("error while executing command %q: %v", cmd.Name(), err)
				return
			}
		}
	})

	return nil
}

func (bot *Bot) registerCommands(ctx context.Context) error {
	for _, cmd := range bot.commands {
		err := bot.register(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to register command %q: %w", cmd.Name(), err)
		}
	}

	return nil
}
