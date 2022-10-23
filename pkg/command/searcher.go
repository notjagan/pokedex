package command

import (
	"context"

	"github.com/notjagan/pokedex/pkg/model"
)

type searcher[T model.Localizer] interface {
	Search(context.Context) ([]T, error)
	Value(T) any
}

type pokemonSearcher struct {
	model  *model.Model
	prefix string
	limit  int
}

func (s pokemonSearcher) Search(ctx context.Context) ([]*model.Pokemon, error) {
	return s.model.SearchPokemon(ctx, s.prefix, s.limit)
}

func (pokemonSearcher) Value(pokemon *model.Pokemon) any {
	return pokemon.Name
}

type versionSearcher struct {
	model  *model.Model
	prefix string
	limit  int
}

func (s versionSearcher) Search(ctx context.Context) ([]*model.Version, error) {
	return s.model.SearchVersions(ctx, s.prefix, s.limit)
}

func (versionSearcher) Value(ver *model.Version) any {
	return ver.Name
}

type languageSearcher struct {
	model *model.Model
}

func (s languageSearcher) Search(ctx context.Context) ([]*model.Language, error) {
	return s.model.AllLanguages(ctx)
}

func (languageSearcher) Value(lang *model.Language) any {
	return lang.ISO639
}

type typeSearcher struct {
	model  *model.Model
	prefix string
	limit  int
}

func (s typeSearcher) Search(ctx context.Context) ([]*model.Type, error) {
	return s.model.SearchTypes(ctx, s.prefix, s.limit)
}

func (typeSearcher) Value(typ *model.Type) any {
	return typ.Name
}
