package model

import "context"

type Type struct {
	model *Model

	ID           int    `db:"id"`
	GenerationID int    `db:"generation_id"`
	Name         string `db:"name"`
}

func (typ *Type) LocalizedName(ctx context.Context) (string, error) {
	return typ.model.localizedTypeName(ctx, typ)
}

func (typ *Type) IsUnknown() bool {
	return typ.Name == "unknown"
}

type TypeCombo struct {
	model *Model

	Type1 *Type
	Type2 *Type
}

func (m *Model) NewTypeCombo() *TypeCombo {
	return &TypeCombo{model: m}
}

func (combo *TypeCombo) DefendingEfficacies(ctx context.Context) ([]TypeEfficacy, error) {
	return combo.model.defendingTypeEfficacies(ctx, combo)
}
