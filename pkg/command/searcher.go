package command

import (
	"context"

	"github.com/notjagan/pokedex/pkg/model"
)

type Searcher[T model.Localizer] interface {
	Search(context.Context) ([]T, error)
	Value(T) any
}

type PokemonSearcher struct {
	model  *model.Model
	prefix string
	limit  int
}

func (s PokemonSearcher) Search(ctx context.Context) ([]*model.Pokemon, error) {
	return s.model.SearchPokemon(ctx, s.prefix, s.limit)
}

func (PokemonSearcher) Value(pokemon *model.Pokemon) any {
	return pokemon.Name
}
