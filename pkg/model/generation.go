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
