package model

type Type struct {
	model *Model

	ID   int    `db:"id"`
	Name string `db:"name"`
}

func (typ *Type) IsUnknown() bool {
	return typ.Name == "unknown"
}
