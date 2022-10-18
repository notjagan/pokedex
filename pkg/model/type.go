package model

type Type struct {
	model *Model

	ID   int    `db:"id"`
	Name string `db:"name"`
}
