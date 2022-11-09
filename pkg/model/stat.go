package model

import (
	"context"
	"errors"
	"fmt"
)

type Stat struct {
	model *Model

	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (stat *Stat) LocalizedName(ctx context.Context) (string, error) {
	return stat.model.statLocalizedName(ctx, stat)
}

type PokemonStats map[int]int

var ErrNoStatFound = errors.New("could not find stat")

func (ps PokemonStats) baseStat(stat Stat) (int, error) {
	baseStat, ok := ps[stat.ID]
	if !ok {
		return 0, fmt.Errorf("pokemon has no stat with id %q: %w", stat.ID, ErrNoStatFound)
	}

	return baseStat, nil
}
