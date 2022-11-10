package command

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type Emojis map[string]*discordgo.Emoji

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
