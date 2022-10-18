package model

import "context"

type Pokemon struct {
	model *Model

	ID        int    `db:"id"`
	Name      string `db:"name"`
	SpeciesID int    `db:"pokemon_species_id"`
}

func (pokemon *Pokemon) LocalizedName(ctx context.Context) (string, error) {
	return pokemon.model.localizedPokemonName(ctx, pokemon)
}

func (pokemon *Pokemon) PokemonMoves(ctx context.Context, methods []*LearnMethod, maxLevel *int, limit *int) ([]PokemonMove, error) {
	var lvl int
	if maxLevel == nil {
		lvl = 100
	} else {
		lvl = *maxLevel
	}

	var lim int
	if limit == nil {
		lim = -1
	} else {
		lim = *limit
	}

	return pokemon.model.pokemonMoves(ctx, pokemon, methods, lvl, lim)
}
