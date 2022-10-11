package model

type LocalizationCode string

const (
	LocalizationCodeEnglish LocalizationCode = "en"
)

type Language struct {
	model *Model

	ID     int              `db:"id"`
	ISO639 LocalizationCode `db:"iso639"`
}
