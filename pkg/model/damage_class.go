package model

type DamageClass struct {
	model *Model

	ID   int    `db:"id"`
	Name string `db:"name"`
}
