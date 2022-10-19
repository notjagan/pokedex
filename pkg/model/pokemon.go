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
