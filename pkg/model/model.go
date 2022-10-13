package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Model struct {
	db *sqlx.DB

	language   *Language
	generation *Generation
}

func New(ctx context.Context, dbPath string) (*Model, error) {
	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to read from database: %w", err)
	}
	return &Model{db: db}, nil
}

func (m *Model) Close() error {
	return m.db.Close()
}

var ErrUnsetLanguage = errors.New("model language is nil")

func (m *Model) SetLanguageByLocalizationCode(ctx context.Context, code LocalizationCode) error {
	lang := Language{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT id, iso639
		FROM pokemon_v2_language
		WHERE iso639 = ?
	`, code).StructScan(&lang)
	if err != nil {
		return fmt.Errorf("locale %q not found: %w", code, err)
	}

	m.language = &lang

	return nil
}

func (m *Model) SetLanguageByLocale(ctx context.Context, locale discordgo.Locale) error {
	code, err := LocaleToLocalizationCode(locale)
	if err != nil {
		return fmt.Errorf("error while decoding preferred locale: %w", err)
	}

	return m.SetLanguageByLocalizationCode(ctx, code)
}

var ErrUnsetGeneration = errors.New("model generation is nil")

func (m *Model) SetGenerationByID(ctx context.Context, id int) error {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT id
		FROM pokemon_v2_generation
		WHERE id = ?
	`, id).StructScan(&gen)
	if err != nil {
		return fmt.Errorf("generation number %v not found: %w", id, err)
	}

	m.generation = &gen

	return nil
}

func (m *Model) SetGeneration(ctx context.Context, gen *Generation) {
	m.generation = gen
}

func (m *Model) LatestGeneration(ctx context.Context) (*Generation, error) {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT MAX(id) AS id
		FROM pokemon_v2_generation
	`).StructScan(&gen)
	if err != nil {
		return nil, fmt.Errorf("could not find latest generation")
	}

	return &gen, nil
}

var ErrWrongGeneration = errors.New("selected resource does not exist in the current generation")

func (m *Model) checkPokemonGeneration(ctx context.Context, pokemon *Pokemon) error {
	if m.generation == nil {
		return ErrUnsetGeneration
	}

	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT generation_id AS id
		FROM pokemon_v2_pokemonspecies
		WHERE id = ?
	`, pokemon.SpeciesID).StructScan(&gen)
	if err != nil {
		return fmt.Errorf("could not find generation for pokemon: %w", err)
	}

	if m.generation.ID < gen.ID {
		return fmt.Errorf("pokemon does not exist in the current generation: %w", ErrWrongGeneration)
	}

	return nil
}

func (m *Model) PokemonById(ctx context.Context, id int) (*Pokemon, error) {
	pokemon := Pokemon{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT id, name, pokemon_species_id
		FROM pokemon_v2_pokemon
		WHERE id = ?
	`, id).StructScan(&pokemon)
	if err != nil {
		return nil, fmt.Errorf("no matching pokemon found: %w", err)
	}

	err = m.checkPokemonGeneration(ctx, &pokemon)
	if err != nil {
		return nil, fmt.Errorf("invalid pokemon for generation: %w", err)
	}

	return &pokemon, nil
}

func (m *Model) PokemonByName(ctx context.Context, name string) (*Pokemon, error) {
	pokemon := Pokemon{model: m}
	err := m.db.QueryRowxContext(ctx, `
		SELECT id, name, pokemon_species_id
		FROM pokemon_v2_pokemon
		WHERE name = ?
	`, name).StructScan(&pokemon)
	if err != nil {
		return nil, fmt.Errorf("no matching pokemon found: %w", err)
	}

	err = m.checkPokemonGeneration(ctx, &pokemon)
	if err != nil {
		return nil, fmt.Errorf("invalid pokemon for generation: %w", err)
	}

	return &pokemon, nil
}

func (m *Model) LocalizedPokemonName(ctx context.Context, pokemon *Pokemon) (string, error) {
	if m.language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx, `
		SELECT name
		FROM pokemon_v2_pokemonspeciesname
		WHERE pokemon_species_id = ? AND language_id = ?
	`, pokemon.SpeciesID, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find name for pokemon %q in locale %q: %w",
			pokemon.Name,
			m.language.ISO639,
			err,
		)
	}

	return name, nil
}
