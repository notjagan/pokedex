package model

import (
	"context"
	"fmt"
)

type Move struct {
	model *Model

	ID            int    `db:"id"`
	Power         *int   `db:"power"`
	PP            int    `db:"pp"`
	Accuracy      *int   `db:"accuracy"`
	DamageClassID int    `db:"move_damage_class_id"`
	TypeID        int    `db:"type_id"`
	Name          string `db:"name"`

	typ   *Type
	class *DamageClass
}

func (move *Move) Type(ctx context.Context) (*Type, error) {
	if move.typ == nil {
		typ, err := move.model.typeByID(ctx, move.TypeID)
		if err != nil {
			return nil, fmt.Errorf("error while getting type: %w", err)
		}
		move.typ = typ
	}

	return move.typ, nil
}

func (move *Move) DamageClass(ctx context.Context) (*DamageClass, error) {
	if move.class == nil {
		class, err := move.model.damageClassByID(ctx, move.DamageClassID)
		if err != nil {
			return nil, fmt.Errorf("error while getting damage class: %w", err)
		}
		move.class = class
	}

	return move.class, nil
}

func (move *Move) LocalizedName(ctx context.Context) (string, error) {
	return move.model.localizedMoveName(ctx, move)
}
