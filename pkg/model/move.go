package model

import (
	"context"
	"fmt"
)

type Move struct {
	model *Model

	ID            int    `db:"id"`
	Power         *int   `db:"power"`
	PP            *int   `db:"pp"`
	Accuracy      *int   `db:"accuracy"`
	DamageClassID int    `db:"move_damage_class_id"`
	TypeID        int    `db:"type_id"`
	Name          string `db:"name"`

	typ   *Type
	class *DamageClass
}

func (move *Move) applyChanges(changes []MoveChange) {
	for _, change := range changes {
		if change.Power != nil {
			move.Power = change.Power
		}

		if change.PP != nil {
			move.PP = change.PP
		}

		if change.Accuracy != nil {
			move.Accuracy = change.Accuracy
		}

		if change.TypeID != nil {
			move.TypeID = *change.TypeID
		}
	}
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

type PokemonMove struct {
	model *Model

	*Move
	Level         int `db:"level"`
	MoveID        int `db:"move_id"`
	LearnMethodID int `db:"move_learn_method_id"`

	learnMethod *LearnMethod
}

func (pm *PokemonMove) LearnMethod(ctx context.Context) (*LearnMethod, error) {
	if pm.learnMethod == nil {
		method, err := pm.model.learnMethodByID(ctx, pm.LearnMethodID)
		if err != nil {
			return nil, fmt.Errorf("error while getting learn method for pokemon move: %w", err)
		}
		pm.learnMethod = method
	}

	return pm.learnMethod, nil
}
