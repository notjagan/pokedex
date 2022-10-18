package model

import (
	"context"
)

type Generation struct {
	model *Model

	ID int
}

func (gen *Generation) HasPokemon(ctx context.Context, pokemon *Pokemon) (bool, error) {
	return gen.model.generationHasPokemon(ctx, gen, pokemon)
}

func (gen *Generation) LocalizedName(ctx context.Context) (string, error) {
	return gen.model.localizedGenerationName(ctx, gen)
}
