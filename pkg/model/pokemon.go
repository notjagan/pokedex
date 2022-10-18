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

func (pokemon *Pokemon) PokemonMoves(ctx context.Context, methods []*LearnMethod) ([]PokemonMove, error) {
	return pokemon.model.pokemonMoves(ctx, pokemon, methods)
}
