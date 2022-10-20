package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type Model struct {
	db *sqlx.DB

	language   *Language
	Generation *Generation
}

func New(ctx context.Context, dbPath string) (*Model, error) {
	db, err := sqlx.Open("sqlite3", fmt.Sprintf("file:%s?mode=ro", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.PingContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to read from database: %w", err)
	}
	return &Model{db: db}, nil
}

func (m *Model) Close() error {
	return m.db.Close()
}

var ErrUnsetLanguage = errors.New("model language is nil")

func (m *Model) languageByLocalizationCode(ctx context.Context, code LocalizationCode) (*Language, error) {
	lang := Language{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, iso639
		FROM pokemon_v2_language
		WHERE iso639 = ?
	`, code).StructScan(&lang)
	if err != nil {
		return nil, fmt.Errorf("localization code %q not found: %w", code, err)
	}
	return &lang, nil
}

func (m *Model) SetLanguageByLocalizationCode(ctx context.Context, code LocalizationCode) error {
	lang, err := m.languageByLocalizationCode(ctx, code)
	if err != nil {
		return fmt.Errorf("error while getting language: %w", err)
	}
	m.language = lang

	return nil
}

func (m *Model) SetLanguageByLocale(ctx context.Context, locale discordgo.Locale) error {
	code, err := LocaleToLocalizationCode(locale)
	if err != nil {
		code, err = LocaleToLocalizationCode(discordgo.EnglishUS)
		if err != nil {
			return fmt.Errorf("error while decoding preferred locale: error while decoding default locale: %w", err)
		}
	}

	return m.SetLanguageByLocalizationCode(ctx, code)
}

var ErrUnsetGeneration = errors.New("model generation is nil")

func (m *Model) SetGenerationByID(ctx context.Context, id int) error {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id
		FROM pokemon_v2_generation
		WHERE id = ?
	`, id).StructScan(&gen)
	if err != nil {
		return fmt.Errorf("generation number %v not found: %w", id, err)
	}

	m.Generation = &gen

	return nil
}

func (m *Model) EarliestGeneration(ctx context.Context) (*Generation, error) {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT MIN(id) AS id
		FROM pokemon_v2_generation
	`).StructScan(&gen)
	if err != nil {
		return nil, fmt.Errorf("could not find latest generation")
	}

	return &gen, nil
}

func (m *Model) LatestGeneration(ctx context.Context) (*Generation, error) {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT MAX(id) AS id
		FROM pokemon_v2_generation
	`).StructScan(&gen)
	if err != nil {
		return nil, fmt.Errorf("could not find latest generation")
	}

	return &gen, nil
}

var ErrWrongGeneration = errors.New("selected resource does not exist in the current generation")

func (m *Model) generationHasPokemon(ctx context.Context, gen *Generation, pokemon *Pokemon) (bool, error) {
	g := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT generation_id AS id
		FROM pokemon_v2_pokemonspecies
		WHERE id = ?
	`, pokemon.SpeciesID).StructScan(&g)
	if err != nil {
		return false, fmt.Errorf("could not find generation for pokemon: %w", err)
	}

	return gen.ID >= g.ID, nil
}

func (m *Model) validatePokemonGeneration(ctx context.Context, pokemon *Pokemon) error {
	if m.Generation == nil {
		return fmt.Errorf("failed to check if generation has pokemon: %w", ErrUnsetGeneration)
	}

	ok, err := m.Generation.HasPokemon(ctx, pokemon)
	if err != nil {
		return fmt.Errorf("failed to check if generation has pokemon: %w", err)
	} else if !ok {
		return ErrWrongGeneration
	}

	return nil
}

func (m *Model) PokemonById(ctx context.Context, id int) (*Pokemon, error) {
	pokemon := Pokemon{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name, pokemon_species_id
		FROM pokemon_v2_pokemon
		WHERE id = ?
	`, id).StructScan(&pokemon)
	if err != nil {
		return nil, fmt.Errorf("no matching pokemon found: %w", err)
	}

	err = m.validatePokemonGeneration(ctx, &pokemon)
	if err != nil {
		return nil, fmt.Errorf("invalid pokemon for generation: %w", err)
	}

	return &pokemon, nil
}

func (m *Model) PokemonByName(ctx context.Context, name string) (*Pokemon, error) {
	pokemon := Pokemon{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name, pokemon_species_id
		FROM pokemon_v2_pokemon
		WHERE name = ?
	`, name).StructScan(&pokemon)
	if err != nil {
		return nil, fmt.Errorf("no matching pokemon found: %w", err)
	}

	err = m.validatePokemonGeneration(ctx, &pokemon)
	if err != nil {
		return nil, fmt.Errorf("invalid pokemon for generation: %w", err)
	}

	return &pokemon, nil
}

func (m *Model) localizedPokemonName(ctx context.Context, pokemon *Pokemon) (string, error) {
	if m.language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_pokemonspeciesname
		WHERE pokemon_species_id = ? AND language_id = ?
	`, pokemon.SpeciesID, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for pokemon %q for language with code %q: %w",
			pokemon.Name,
			m.language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) AllLanguages(ctx context.Context) ([]*Language, error) {
	langs := make([]*Language, len(AllLocalizationCodes))

	for i, code := range AllLocalizationCodes {
		lang, err := m.languageByLocalizationCode(ctx, code)
		if err != nil {
			return nil, fmt.Errorf("error while getting all languages: %w", err)
		}
		langs[i] = lang
	}

	return langs, nil
}

func (m *Model) localizedLanguageName(ctx context.Context, lang *Language) (string, error) {
	if m.language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_languagename
		WHERE language_id = ? AND local_language_id = ?
	`, lang.ID, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("error while getting localized name for language with code %q: %w", lang.ISO639, err)
	}

	return name, nil
}

func (m *Model) searchPokemonMoves(
	ctx context.Context,
	pokemon *Pokemon,
	methods []*LearnMethod,
	maxLevel *int,
	top *int,
	limit int,
	offset int,
) ([]PokemonMove, bool, error) {
	if m.Generation == nil {
		return nil, false, ErrUnsetGeneration
	}

	var lvl int
	if maxLevel == nil {
		lvl = 100
	} else {
		lvl = *maxLevel
	}

	var t int
	if top == nil {
		t = -1
	} else {
		t = *top
	}

	ids := make([]int, len(methods))
	for i, method := range methods {
		ids[i] = method.ID
	}

	query, args, err := sqlx.In(
		/* sql */ `
		SELECT id, level, move_id, move_learn_method_id FROM (
			SELECT *, rank() OVER (ORDER BY level DESC) AS r FROM (
				SELECT MIN(m.id) as id, m.level, m.move_id, m.move_learn_method_id
				FROM pokemon_v2_pokemonmove m
				JOIN pokemon_v2_versiongroup v
					ON m.version_group_id = v.id
				WHERE m.pokemon_id = ? AND v.generation_id = ? AND m.level <= ? AND m.move_learn_method_id IN (?)
				GROUP BY m.move_id
			)
		)
		WHERE ? < 0 OR r <= ?
		ORDER BY r DESC
		LIMIT ? OFFSET ?
	`, pokemon.ID, m.Generation.ID, lvl, ids, t, t, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("error while constructing query: %w", err)
	}

	moves := []PokemonMove{}
	err = m.db.SelectContext(ctx, &moves, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("error while getting moves for pokemon in generation: %w", err)
	}

	for i := range moves {
		moves[i].model = m
	}

	var hasNext bool
	if len(moves) == limit+1 {
		moves = moves[:limit]
		hasNext = true
	} else {
		hasNext = false
	}

	return moves, hasNext, nil
}

func (m *Model) moveByID(ctx context.Context, ID int) (*Move, error) {
	move := Move{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, power, pp, accuracy, move_damage_class_id, type_id, name
		FROM pokemon_v2_move
		WHERE id = ?
	`, ID).StructScan(&move)
	if err != nil {
		return nil, fmt.Errorf("no matching move found: %w", err)
	}

	return &move, nil
}

func (m *Model) typeByID(ctx context.Context, ID int) (*Type, error) {
	typ := Type{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name
		FROM pokemon_v2_type
		WHERE id = ?
	`, ID).StructScan(&typ)
	if err != nil {
		return nil, fmt.Errorf("no matching type found: %w", err)
	}

	return &typ, nil
}

func (m *Model) learnMethodByID(ctx context.Context, ID int) (*LearnMethod, error) {
	method := LearnMethod{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name
		FROM pokemon_v2_movelearnmethod
		WHERE id = ?
	`, ID).StructScan(&method)
	if err != nil {
		return nil, fmt.Errorf("no matching learn method found: %w", err)
	}

	return &method, nil
}

func (m *Model) learnMethodByName(ctx context.Context, name LearnMethodName) (*LearnMethod, error) {
	method := LearnMethod{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name
		FROM pokemon_v2_movelearnmethod
		WHERE name = ?
	`, name).StructScan(&method)
	if err != nil {
		return nil, fmt.Errorf("no matching learn method found: %w", err)
	}

	return &method, nil
}

func (m *Model) LearnMethodsByName(ctx context.Context, names []LearnMethodName) ([]*LearnMethod, error) {
	methods := make([]*LearnMethod, len(names))
	for i, name := range names {
		method, err := m.learnMethodByName(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("failed to get learn method for name %q: %w", name, err)
		}
		methods[i] = method
	}

	return methods, nil
}

func (m *Model) damageClassByID(ctx context.Context, ID int) (*DamageClass, error) {
	class := DamageClass{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name
		FROM pokemon_v2_movedamageclass
		WHERE id = ?
	`, ID).StructScan(&class)
	if err != nil {
		return nil, fmt.Errorf("no matching damage class found: %w", err)
	}

	return &class, nil
}

func (m *Model) localizedMoveName(ctx context.Context, move *Move) (string, error) {
	if m.language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_movename
		WHERE move_id = ? AND language_id = ?
	`, move.ID, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for move %q for language with code %q: %w",
			move.Name,
			m.language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) localizedGenerationName(ctx context.Context, gen *Generation) (string, error) {
	if m.language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_generationname
		WHERE generation_id = ? AND language_id = ?
	`, gen.ID, m.language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for generation %d for language with code %q: %w",
			gen.ID,
			m.language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) SearchPokemon(ctx context.Context, prefix string, limit int) ([]*Pokemon, error) {
	if m.language == nil {
		return nil, ErrUnsetLanguage
	}
	if m.Generation == nil {
		return nil, ErrUnsetGeneration
	}

	pattern := fmt.Sprintf("%s%%", prefix)
	var ps []*Pokemon
	err := m.db.SelectContext(ctx, &ps,
		/* sql */ `
		SELECT MIN(p.id) as id, p.name, p.pokemon_species_id
		FROM pokemon_v2_pokemon p
		JOIN pokemon_v2_pokemonspeciesname n
			ON p.pokemon_species_id = n.pokemon_species_id
		JOIN pokemon_v2_pokemonspecies s
			ON p.pokemon_species_id = s.id
		WHERE n.name LIKE ? AND n.language_id = ? AND s.generation_id <= ?
		GROUP BY p.pokemon_species_id
		ORDER BY n.name ASC
		LIMIT ?
	`, pattern, m.language.ID, m.Generation.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("error while getting pokemon with prefix: %w", err)
	}

	for i := range ps {
		ps[i].model = m
	}

	return ps, nil
}
