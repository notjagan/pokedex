package model

import "context"

type Ability struct {
	model *Model

	ID           int    `db:"id"`
	IsMainSeries bool   `db:"is_main_series"`
	GenerationID int    `db:"generation_id"`
	Name         string `db:"name"`
}

func (ability *Ability) LocalizedName(ctx context.Context) (string, error) {
	return ability.model.abilityLocalizedName(ctx, ability)
}

type PokemonAbility struct {
	model *Model

	*Ability
	IsHidden  bool `db:"is_hidden"`
	AbilityID int  `db:"ability_id"`
}
