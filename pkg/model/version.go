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

	vg *VersionGroup
}

func (ver *Version) VersionGroup(ctx context.Context) (*VersionGroup, error) {
	if ver.vg == nil {
		vg, err := ver.model.versionGroupByID(ctx, ver.VersionGroupID)
		if err != nil {
			return nil, fmt.Errorf("error while getting version group for version: %w", err)
		}
		ver.vg = vg
	}

	return ver.vg, nil
}

func (ver *Version) Generation(ctx context.Context) (*Generation, error) {
	vg, err := ver.VersionGroup(ctx)
	if err != nil {
		return nil, err
	}

	gen, err := vg.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting generation for version %q: %w", ver.Name, err)
	}

	return gen, nil
}

func (ver *Version) LocalizedName(ctx context.Context) (string, error) {
	return ver.model.localizedVersionName(ctx, ver)
}

func (ver *Version) HasPokemon(ctx context.Context, pokemon *Pokemon) (bool, error) {
	return ver.model.versionHasPokemon(ctx, ver, pokemon)
}

func (ver *Version) HasMove(ctx context.Context, move *Move) (bool, error) {
	return ver.model.versionHasMove(ctx, ver, move)
}
