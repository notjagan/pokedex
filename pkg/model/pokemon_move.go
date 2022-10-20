package model

import (
	"context"
	"fmt"
)

type PokemonMove struct {
	model *Model

	ID            int `db:"id"`
	Level         int `db:"level"`
	MoveID        int `db:"move_id"`
	LearnMethodID int `db:"move_learn_method_id"`

	move        *Move
	learnMethod *LearnMethod
}

func (pm *PokemonMove) Move(ctx context.Context) (*Move, error) {
	if pm.move == nil {
		move, err := pm.model.moveByID(ctx, pm.MoveID)
		if err != nil {
			return nil, fmt.Errorf("error while getting move: %w", err)
		}
		pm.move = move
	}

	return pm.move, nil
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
