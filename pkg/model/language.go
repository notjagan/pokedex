package model

import (
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type LocalizationCode string

const (
	LocalizationCodeEnglish LocalizationCode = "en"
	UnknownLocalizationCode LocalizationCode = ""
)

type Language struct {
	model *Model

	ID     int              `db:"id"`
	ISO639 LocalizationCode `db:"iso639"`
}

var ErrUnrecognizedLocale = errors.New("could not identify locale")

func LocaleToLocalizationCode(locale discordgo.Locale) (LocalizationCode, error) {
	switch locale {
	case discordgo.EnglishUS:
		return LocalizationCodeEnglish, nil
	default:
		return UnknownLocalizationCode, fmt.Errorf("unrecognized locale %q: %w", locale, ErrUnrecognizedLocale)
	}
}
