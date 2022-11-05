package model

import (
	"context"
	"fmt"

	"github.com/notjagan/pokedex/pkg/model/sprite"
)

type Pokemon struct {
	model *Model

	ID        int    `db:"id"`
	Name      string `db:"name"`
	SpeciesID int    `db:"pokemon_species_id"`

	sprites *sprite.PokemonSprites
}

func (pokemon *Pokemon) LocalizedName(ctx context.Context) (string, error) {
	return pokemon.model.localizedPokemonName(ctx, pokemon)
}

func (pokemon *Pokemon) SearchPokemonMoves(
	ctx context.Context,
	methods []*LearnMethod,
	maxLevel *int,
	top *int,
	limit int,
	offset int,
) ([]PokemonMove, bool, error) {
	return pokemon.model.searchPokemonMoves(ctx, pokemon, methods, maxLevel, top, limit, offset)
}

func (pokemon *Pokemon) TypeCombo(ctx context.Context) (*TypeCombo, error) {
	return pokemon.model.pokemonTypeCombo(ctx, pokemon)
}

func (pokemon *Pokemon) Sprites(ctx context.Context) (*sprite.PokemonSprites, error) {
	if pokemon.sprites == nil {
		sprites, err := pokemon.model.pokemonSprites(ctx, pokemon)
		if err != nil {
			return nil, fmt.Errorf("error while getting sprites for pokemon: %w", err)
		}
		pokemon.sprites = sprites
	}

	return pokemon.sprites, nil
}
