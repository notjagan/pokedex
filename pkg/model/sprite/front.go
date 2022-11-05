package sprite

type Front struct {
	Default          Sprite  `json:"front_default"`
	Gray             *Sprite `json:"front_gray"`
	Female           *Sprite `json:"front_female"`
	Shiny            *Sprite `json:"front_shiny"`
	ShinyFemale      *Sprite `json:"front_shiny_female"`
	Transparent      *Sprite `json:"front_transparent"`
	ShinyTransparent *Sprite `json:"front_shiny_transparent"`
}
