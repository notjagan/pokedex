package model

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Model struct {
	db *sqlx.DB

	language *Language
}

func Open(ctx context.Context, dbPath string) (*Model, error) {
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	m := &Model{db: db}

	err = m.SetLanguage(ctx, LocalizationCodeEnglish)
	if err != nil {
		return nil, fmt.Errorf("default locale not found: %w", err)
	}

	return m, nil
}

func (m *Model) Close() error {
	return m.db.Close()
}

func (m *Model) SetLanguage(ctx context.Context, code LocalizationCode) error {
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

	return &pokemon, nil
}

func (m *Model) LocalizedPokemonName(ctx context.Context, p *Pokemon) (string, error) {
	var name string
	err := m.db.QueryRowxContext(ctx, `
		SELECT name
		FROM pokemon_v2_pokemonspeciesname
		WHERE pokemon_species_id = ? AND language_id = ?
	`, p.SpeciesId, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find name for pokemon %q in locale %q: %w",
			p.Name,
			m.language.ISO639,
			err,
		)
	}

	return name, nil
}
