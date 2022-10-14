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
	Config         config.Config
	Session        *discordgo.Session
	CommandBuilder *command.Builder
	Models         map[string]*model.Model
}

func New(ctx context.Context, config config.Config) (*Bot, error) {
	mdl, err := model.New(ctx, config.DB.Path)
	if err != nil {
		return nil, fmt.Errorf("error while creating model for command builder: %w", err)
	}

	return &Bot{
		Config:         config,
		CommandBuilder: &command.Builder{Model: mdl},
		Models:         make(map[string]*model.Model),
	}, nil
}

func (bot *Bot) Close() error {
	log.Println("Shutting down.")
	return bot.Session.Close()
}

func (bot *Bot) addGuild(ctx context.Context, guild *discordgo.Guild) error {
	mdl, err := model.New(ctx, bot.Config.DB.Path)
	if err != nil {
		return fmt.Errorf("error while instantiating model for guild %q: %w", guild.Name, err)
	}
	bot.Models[guild.ID] = mdl

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
	delete(bot.Models, guild.ID)
}

var ErrNoMatchingModel = errors.New("no matching model")

func (bot *Bot) model(guild *discordgo.Guild) (*model.Model, error) {
	model, ok := bot.Models[guild.ID]
	if !ok {
		return nil, fmt.Errorf("could not find model for guild %q: %w", guild.Name, ErrNoMatchingModel)
	}

	return model, nil
}

func (bot *Bot) initialize(ctx context.Context) error {
	sess, err := discordgo.New("Bot " + bot.Config.Discord.Token)
	if err != nil {
		return fmt.Errorf("failed to instantiate discord bot: %w", err)
	}
	bot.Session = sess

	err = bot.Session.Open()
	if err != nil {
		return fmt.Errorf("failed to start discord session: %w", err)
	}

	bot.Session.AddHandler(func(_ *discordgo.Session, create *discordgo.GuildCreate) {
		err := bot.addGuild(ctx, create.Guild)
		if err != nil {
			log.Printf("failed to add guild %q: %v", create.Guild.Name, err)
		}
	})
	bot.Session.AddHandler(func(_ *discordgo.Session, delete *discordgo.GuildDelete) {
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
	_, err := bot.Session.ApplicationCommandCreate(bot.Session.State.User.ID, "", cmd.ApplicationCommand())
	if err != nil {
		return fmt.Errorf("failed to create command %q: %w", cmd.Name(), err)
	}
	bot.Session.AddHandler(func(sess *discordgo.Session, interaction *discordgo.InteractionCreate) {
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
	cmds, err := bot.CommandBuilder.All(ctx)
	if err != nil {
		return fmt.Errorf("error while getting all commands for bot: %w", err)
	}

	for _, cmd := range cmds {
		err := bot.register(ctx, cmd)
		if err != nil {
			return fmt.Errorf("failed to register command %q: %w", cmd.Name(), err)
		}
	}

	return nil
}
