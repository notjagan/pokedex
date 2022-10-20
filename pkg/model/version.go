package model

import (
	"context"
	"fmt"
)

type VersionName string

const (
	VersionNameSword = "sword"
)

type Version struct {
	model *Model

	ID             int    `db:"id"`
	VersionGroupID int    `db:"version_group_id"`
	Name           string `db:"name"`

	gen *Generation
}

func (ver *Version) Generation(ctx context.Context) (*Generation, error) {
	if ver.gen == nil {
		g, err := ver.model.versionGeneration(ctx, ver)
		if err != nil {
			return nil, fmt.Errorf("error while getting generation for version: %w", err)
		}
		ver.gen = g
	}

	return ver.gen, nil
}

func (ver *Version) LocalizedName(ctx context.Context) (string, error) {
	return ver.model.localizedVersionName(ctx, ver)
}

func (ver *Version) HasPokemon(ctx context.Context, pokemon *Pokemon) (bool, error) {
	return ver.model.versionHasPokemon(ctx, ver, pokemon)
}
