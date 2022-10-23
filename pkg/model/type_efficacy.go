package model

import (
	"context"
	"fmt"
)

type EfficacyLevel int

const (
	DoubleSuperEffective   EfficacyLevel = 400
	SuperEffective         EfficacyLevel = 200
	NormalEffective        EfficacyLevel = 100
	NotVeryEffective       EfficacyLevel = 50
	DoubleNotVeryEffective EfficacyLevel = 25
	Immune                 EfficacyLevel = 0
)

type TypeEfficacy struct {
	model *Model

	DamageFactor   int `db:"damage_factor"`
	OpposingTypeID int `db:"opposing_type_id"`

	opposingType *Type
}

func (te *TypeEfficacy) EfficacyLevel() EfficacyLevel {
	return EfficacyLevel(te.DamageFactor)
}

func (te *TypeEfficacy) OpposingType(ctx context.Context) (*Type, error) {
	if te.opposingType == nil {
		typ, err := te.model.typeByID(ctx, te.OpposingTypeID)
		if err != nil {
			return nil, fmt.Errorf("could not get type for type efficacy: %w", err)
		}
		te.opposingType = typ
	}

	return te.opposingType, nil
}
