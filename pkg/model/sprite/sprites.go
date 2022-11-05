package sprite

type Sprites struct {
	Front
	*Back
}

type PokemonSprites struct {
	Sprites
	Versions map[string]map[string]Sprites `json:"versions"`
	Other    map[string]Sprites            `json:"other"`
}
