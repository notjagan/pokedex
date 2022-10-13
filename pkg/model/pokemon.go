package model

import "context"

type Pokemon struct {
	model *Model

	ID        int    `db:"id"`
	Name      string `db:"name"`
	SpeciesID int    `db:"pokemon_species_id"`
}

func (p *Pokemon) LocalizedName(ctx context.Context) (string, error) {
	return p.model.LocalizedPokemonName(ctx, p)
}
