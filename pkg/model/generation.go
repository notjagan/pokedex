package model

import (
	"context"
)

type Generation struct {
	model *Model

	ID int
}

func (gen *Generation) LocalizedName(ctx context.Context) (string, error) {
	return gen.model.localizedGenerationName(ctx, gen)
}
