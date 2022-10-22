package model

type MoveChange struct {
	model *Model

	Power          *int `db:"power"`
	PP             *int `db:"pp"`
	Accuracy       *int `db:"accuracy"`
	TypeID         *int `db:"type_id"`
	VersionGroupID int  `db:"version_group_id"`
	MoveID         int  `db:"move_id"`
}
