package model

import (
	"context"
	"fmt"
)

type VersionGroup struct {
	model *Model

	ID           int    `db:"id"`
	GenerationID int    `db:"generation_id"`
	Name         string `db:"name"`

	gen *Generation
}

func (vg *VersionGroup) Generation(ctx context.Context) (*Generation, error) {
	if vg.gen == nil {
		gen, err := vg.model.GenerationByID(ctx, vg.GenerationID)
		if err != nil {
			return nil, fmt.Errorf("error while getting generation for version group %q: %w", vg.Name, err)
		}
		vg.gen = gen
	}

	return vg.gen, nil
}
