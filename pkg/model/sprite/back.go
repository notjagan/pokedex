package sprite

type Back struct {
	Default          Sprite  `json:"back_default"`
	Gray             *Sprite `json:"back_gray"`
	Female           *Sprite `json:"back_female"`
	Shiny            *Sprite `json:"back_shiny"`
	ShinyFemale      *Sprite `json:"back_shiny_female"`
	Transparent      *Sprite `json:"back_transparent"`
	ShinyTransparent *Sprite `json:"back_shiny_transparent"`
}
