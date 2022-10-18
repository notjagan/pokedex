package model

type LearnMethodName string

const (
	LevelUp LearnMethodName = "level-up"
	Egg     LearnMethodName = "egg"
)

type LearnMethod struct {
	model *Model

	ID   int
	Name string
}
