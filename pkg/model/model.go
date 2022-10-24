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

	Language *Language
	Version  *Version
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
	m.Language = lang

	return nil
}

func (m *Model) SetLanguageByLocale(ctx context.Context, locale discordgo.Locale) error {
	code, err := LocaleToLocalizationCode(locale)
	if err != nil {
		code, err = LocaleToLocalizationCode(discordgo.EnglishUS)
		if err != nil {
			return fmt.Errorf("error while decoding preferred locale: %w",
				fmt.Errorf("error while decoding default locale: %w", err),
			)
		}
	}

	return m.SetLanguageByLocalizationCode(ctx, code)
}

func (m *Model) versionByName(ctx context.Context, name string) (*Version, error) {
	ver := Version{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, version_group_id, name
		FROM pokemon_v2_version
		WHERE name = ?
	`, name).StructScan(&ver)
	if err != nil {
		return nil, fmt.Errorf("version %q not found: %w", name, err)
	}

	return &ver, nil
}

var ErrUnsetVersion = errors.New("model version is nil")

func (m *Model) SetVersionByName(ctx context.Context, name string) error {
	ver, err := m.versionByName(ctx, name)
	if err != nil {
		return fmt.Errorf("version %q not found: %w", name, err)
	}

	m.Version = ver

	return nil
}

var ErrWrongGeneration = errors.New("selected resource does not exist in the current generation")

func (m *Model) versionGeneration(ctx context.Context, ver *Version) (*Generation, error) {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT generation_id as id
		FROM pokemon_v2_versiongroup
		WHERE id = ?
	`, ver.VersionGroupID).StructScan(&gen)
	if err != nil {
		return nil, fmt.Errorf("could not find generation for version %q: %w", ver.Name, err)
	}

	return &gen, nil
}

func (m *Model) latestGeneration(ctx context.Context) (*Generation, error) {
	gen := Generation{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT max(id) as id
		FROM pokemon_v2_generation
	`).StructScan(&gen)
	if err != nil {
		return nil, fmt.Errorf("could not get latest generation: %w", err)
	}

	return &gen, nil
}

func (m *Model) versionHasPokemon(ctx context.Context, ver *Version, pokemon *Pokemon) (bool, error) {
	gen, err := ver.Generation(ctx)
	if err != nil {
		return false, fmt.Errorf("error while getting generation for queried version: %w", err)
	}

	var exists bool
	err = m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT EXISTS (
			SELECT 1
			FROM pokemon_v2_pokemonspecies
			WHERE id = ? AND generation_id <= ?
		)
	`, pokemon.SpeciesID, gen.ID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error while querying pokemon generation: %w", err)
	}

	return exists, nil
}

func (m *Model) versionHasMove(ctx context.Context, ver *Version, move *Move) (bool, error) {
	gen, err := ver.Generation(ctx)
	if err != nil {
		return false, fmt.Errorf("error while getting generation for queried version: %w", err)
	}

	var exists bool
	err = m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT EXISTS (
			SELECT 1
			FROM pokemon_v2_move
			WHERE id = ? AND generation_id <= ?
		)
	`, move.ID, gen.ID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error while querying move generation: %w", err)
	}

	return exists, nil
}

func (m *Model) validatePokemonVersion(ctx context.Context, pokemon *Pokemon) error {
	if m.Version == nil {
		return fmt.Errorf("failed to check if version has pokemon: %w", ErrUnsetVersion)
	}

	ok, err := m.Version.HasPokemon(ctx, pokemon)
	if err != nil {
		return fmt.Errorf("failed to check if version has pokemon: %w", err)
	} else if !ok {
		return ErrWrongGeneration
	}

	return nil
}

func (m *Model) validateMoveVersion(ctx context.Context, move *Move) error {
	if m.Version == nil {
		return fmt.Errorf("failed to check if version has move: %w", ErrUnsetVersion)
	}

	ok, err := m.Version.HasMove(ctx, move)
	if err != nil {
		return fmt.Errorf("failed to check if version has move: %w", err)
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

	err = m.validatePokemonVersion(ctx, &pokemon)
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

	err = m.validatePokemonVersion(ctx, &pokemon)
	if err != nil {
		return nil, fmt.Errorf("invalid pokemon for generation: %w", err)
	}

	return &pokemon, nil
}

func (m *Model) localizedPokemonName(ctx context.Context, pokemon *Pokemon) (string, error) {
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_pokemonspeciesname
		WHERE pokemon_species_id = ? AND language_id = ?
	`, pokemon.SpeciesID, m.Language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for pokemon %q for language with code %q: %w",
			pokemon.Name,
			m.Language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) AllVersions(ctx context.Context) ([]Version, error) {
	var vers []Version
	err := m.db.SelectContext(ctx, &vers,
		/* sql */ `
		SELECT id, version_group_id, name
		FROM pokemon_v2_version
	`)
	if err != nil {
		return nil, fmt.Errorf("error while getting all versions: %w", err)
	}

	for i := range vers {
		vers[i].model = m
	}

	return vers, nil
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
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_languagename
		WHERE language_id = ? AND local_language_id = ?
	`, lang.ID, m.Language.ID).Scan(&name)
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
	if m.Version == nil {
		return nil, false, ErrUnsetVersion
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
			SELECT MIN(id) as id, level, move_id, move_learn_method_id, rank() OVER (ORDER BY level DESC) AS r
			FROM pokemon_v2_pokemonmove
			WHERE pokemon_id = ? AND version_group_id = ? AND level <= ? AND move_learn_method_id IN (?)
			GROUP BY move_id
		)
		WHERE ? < 0 OR r <= ?
		ORDER BY r DESC
		LIMIT ? OFFSET ?
	`, pokemon.ID, m.Version.VersionGroupID, lvl, ids, t, t, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("error while constructing query: %w", err)
	}

	var moves []PokemonMove
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

func (m *Model) moveChanges(ctx context.Context, moveID int) ([]MoveChange, error) {
	var changes []MoveChange
	err := m.db.SelectContext(ctx, &changes,
		/* sql */ `
		SELECT power, pp, accuracy, type_id, version_group_id, move_id
		FROM pokemon_v2_movechange
		WHERE move_id = ? AND version_group_id > ?
		ORDER BY version_group_id DESC
	`, moveID, m.Version.VersionGroupID)
	if err != nil {
		return nil, fmt.Errorf("could not find move changes for move: %w", err)
	}

	for i := range changes {
		changes[i].model = m
	}

	return changes, nil
}

func (m *Model) moveByID(ctx context.Context, id int) (*Move, error) {
	move := Move{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, power, pp, accuracy, move_damage_class_id, type_id, name
		FROM pokemon_v2_move
		WHERE id = ?
	`, id).StructScan(&move)
	if err != nil {
		return nil, fmt.Errorf("no matching move found: %w", err)
	}

	changes, err := m.moveChanges(ctx, move.ID)
	if err != nil {
		return nil, fmt.Errorf("error while getting move changes: %w", err)
	}

	move.applyChanges(changes)

	return &move, nil
}

func (m *Model) MoveByName(ctx context.Context, name string) (*Move, error) {
	move := Move{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, power, pp, accuracy, move_damage_class_id, type_id, name
		FROM pokemon_v2_move
		WHERE name = ?
	`, name).StructScan(&move)
	if err != nil {
		return nil, fmt.Errorf("no matching move found: %w", err)
	}

	err = m.validateMoveVersion(ctx, &move)
	if err != nil {
		return nil, fmt.Errorf("move not found in version: %w", err)
	}

	changes, err := m.moveChanges(ctx, move.ID)
	if err != nil {
		return nil, fmt.Errorf("error while getting move changes: %w", err)
	}

	move.applyChanges(changes)

	return &move, nil
}

func (m *Model) typeByID(ctx context.Context, id int) (*Type, error) {
	typ := Type{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, generation_id, name
		FROM pokemon_v2_type
		WHERE id = ?
	`, id).StructScan(&typ)
	if err != nil {
		return nil, fmt.Errorf("no matching type found: %w", err)
	}

	return &typ, nil
}

func (m *Model) TypeByName(ctx context.Context, name string) (*Type, error) {
	typ := Type{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, generation_id, name
		FROM pokemon_v2_type
		WHERE name = ?
	`, name).StructScan(&typ)
	if err != nil {
		return nil, fmt.Errorf("no matching type found: %w", err)
	}

	return &typ, nil
}

func (m *Model) learnMethodByID(ctx context.Context, id int) (*LearnMethod, error) {
	method := LearnMethod{model: m}
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT id, name
		FROM pokemon_v2_movelearnmethod
		WHERE id = ?
	`, id).StructScan(&method)
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
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_movename
		WHERE move_id = ? AND language_id = ?
	`, move.ID, m.Language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for move %q for language with code %q: %w",
			move.Name,
			m.Language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) localizedGenerationName(ctx context.Context, gen *Generation) (string, error) {
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_generationname
		WHERE generation_id = ? AND language_id = ?
	`, gen.ID, m.Language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for generation %d for language with code %q: %w",
			gen.ID,
			m.Language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) localizedVersionName(ctx context.Context, ver *Version) (string, error) {
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_versionname
		WHERE version_id = ? AND language_id = ?
	`, ver.ID, m.Language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for version %q for language with code %q: %w",
			ver.Name,
			m.Language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) localizedTypeName(ctx context.Context, typ *Type) (string, error) {
	if m.Language == nil {
		return "", ErrUnsetLanguage
	}

	var name string
	err := m.db.QueryRowxContext(ctx,
		/* sql */ `
		SELECT name
		FROM pokemon_v2_typename
		WHERE type_id = ? AND language_id = ?
	`, typ.ID, m.Language.ID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf(
			"could not find localized name for type %q for language with code %q: %w",
			typ.Name,
			m.Language.ISO639,
			err,
		)
	}

	return name, nil
}

func (m *Model) SearchVersions(ctx context.Context, prefix string, limit int) ([]*Version, error) {
	if m.Language == nil {
		return nil, ErrUnsetLanguage
	}

	pattern := fmt.Sprintf("%s%%", prefix)
	var vers []*Version
	err := m.db.SelectContext(ctx, &vers,
		/* sql */ `
		SELECT v.id, v.version_group_id, v.name
		FROM pokemon_v2_version v
		JOIN pokemon_v2_versionname n
			ON v.id = n.version_id
		WHERE n.name LIKE ? AND n.language_id = ?
		ORDER BY n.name asc
		LIMIT ?
	`, pattern, m.Language.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("error while getting versions with prefix: %w", err)
	}

	for i := range vers {
		vers[i].model = m
	}

	return vers, nil
}

func (m *Model) SearchPokemon(ctx context.Context, prefix string, limit int) ([]*Pokemon, error) {
	if m.Language == nil {
		return nil, ErrUnsetLanguage
	}
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	pattern := fmt.Sprintf("%s%%", prefix)
	var ps []*Pokemon
	err = m.db.SelectContext(ctx, &ps,
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
	`, pattern, m.Language.ID, gen.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("error while getting pokemon with prefix: %w", err)
	}

	for i := range ps {
		ps[i].model = m
	}

	return ps, nil
}

func (m *Model) SearchMoves(ctx context.Context, prefix string, limit int) ([]*Move, error) {
	if m.Language == nil {
		return nil, ErrUnsetLanguage
	}
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	pattern := fmt.Sprintf("%s%%", prefix)
	var moves []*Move
	err = m.db.SelectContext(ctx, &moves,
		/* sql */ `
		SELECT MIN(m.id) as id, m.power, m.pp, m.accuracy, m.move_damage_class_id, m.type_id, m.name
		FROM pokemon_v2_move m
		JOIN pokemon_v2_movename n
			ON m.id = n.move_id
		WHERE n.name LIKE ? AND n.language_id = ? AND m.generation_id <= ?
		GROUP BY n.name
		ORDER BY n.name ASC
		LIMIT ?
	`, pattern, m.Language.ID, gen.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("error while getting moves with prefix: %w", err)
	}

	for i := range moves {
		moves[i].model = m
	}

	return moves, nil
}

func (m *Model) defendingTypeEfficacies(ctx context.Context, combo *TypeCombo) ([]TypeEfficacy, error) {
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	g, err := m.latestGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting latest generation: %w", err)
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	var effs []TypeEfficacy
	if combo.Type2 == nil {
		err = m.db.SelectContext(ctx, &effs,
			/* sql */ `
			SELECT DISTINCT damage_type_id AS opposing_type_id, FIRST_VALUE(damage_factor) OVER (
				PARTITION BY damage_type_id
				ORDER BY e.generation_id ASC
			) as damage_factor
			FROM (
				SELECT damage_factor, damage_type_id, target_type_id, ? as generation_id
				FROM pokemon_v2_typeefficacy
				UNION ALL
				SELECT damage_factor, damage_type_id, target_type_id, generation_id
				FROM pokemon_v2_typeefficacypast
			) e
			JOIN pokemon_v2_type dt
				ON e.damage_type_id = dt.id
			WHERE target_type_id = ? AND dt.generation_id <= ? AND e.generation_id >= ?
			ORDER BY damage_type_id
		`, g.ID, combo.Type1.ID, gen.ID, gen.ID)
		if err != nil {
			return nil, fmt.Errorf(
				"could not get type efficacies against type %q: %w",
				combo.Type1.Name,
				err,
			)
		}
	} else {
		err = m.db.SelectContext(ctx, &effs,
			/* sql */ `
			WITH e AS (
				SELECT damage_factor, damage_type_id, target_type_id, ? as generation_id
				FROM pokemon_v2_typeefficacy
				UNION ALL
				SELECT damage_factor, damage_type_id, target_type_id, generation_id
				FROM pokemon_v2_typeefficacypast
			)
			SELECT DISTINCT e1.damage_type_id AS opposing_type_id, FIRST_VALUE(e1.damage_factor) OVER (
				PARTITION BY e1.damage_type_id
				ORDER BY e1.generation_id ASC
			) * FIRST_VALUE(e2.damage_factor) OVER (
				PARTITION BY e2.damage_type_id
				ORDER BY e2.generation_id ASC
			) / 100 as damage_factor
			FROM e e1
			JOIN e e2
				ON e1.damage_type_id = e2.damage_type_id
			JOIN pokemon_v2_type dt
				ON e1.damage_type_id = dt.id
			WHERE e1.target_type_id = ?
				AND e2.target_type_id = ?
				AND dt.generation_id <= ?
				AND e1.generation_id >= ?
				AND e2.generation_id >= ?
			ORDER BY dt.id
		`, g.ID, combo.Type1.ID, combo.Type2.ID, gen.ID, gen.ID, gen.ID)
		if err != nil {
			return nil, fmt.Errorf("could not get type efficacies against types %q and %q: %w",
				combo.Type1.Name,
				combo.Type2.Name,
				err,
			)
		}
	}

	for i := range effs {
		effs[i].model = m
	}

	return effs, nil
}

func (m *Model) attackingTypeEfficacies(ctx context.Context, typ *Type) ([]TypeEfficacy, error) {
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	g, err := m.latestGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting latest generation: %w", err)
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	var effs []TypeEfficacy
	err = m.db.SelectContext(ctx, &effs,
		/* sql */ `
		SELECT DISTINCT target_type_id AS opposing_type_id, FIRST_VALUE(damage_factor) OVER (
			PARTITION BY target_type_id
			ORDER BY e.generation_id ASC
		) as damage_factor
		FROM (
			SELECT damage_factor, damage_type_id, target_type_id, ? as generation_id
			FROM pokemon_v2_typeefficacy
			UNION ALL
			SELECT damage_factor, damage_type_id, target_type_id, generation_id
			FROM pokemon_v2_typeefficacypast
		) e
		JOIN pokemon_v2_type tt
			ON e.target_type_id = tt.id
		WHERE damage_type_id = ? AND tt.generation_id <= ? AND e.generation_id >= ?
		ORDER BY target_type_id
	`, g.ID, typ.ID, gen.ID, gen.ID)
	if err != nil {
		return nil, fmt.Errorf(
			"could not get type efficacies for type %q: %w",
			typ.Name,
			err,
		)
	}

	for i := range effs {
		effs[i].model = m
	}

	return effs, nil
}

func (m *Model) SearchTypes(ctx context.Context, prefix string, limit int) ([]*Type, error) {
	if m.Language == nil {
		return nil, ErrUnsetLanguage
	}
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	pattern := fmt.Sprintf("%s%%", prefix)
	var types []*Type
	err = m.db.SelectContext(ctx, &types,
		/* sql */ `
		SELECT t.id, t.generation_id, t.name
		FROM pokemon_v2_type t
		JOIN pokemon_v2_typename n
			ON t.id = n.type_id
		WHERE t.generation_id <= ? AND n.name LIKE ? AND n.language_id = ?
		LIMIT ?
	`, gen.ID, pattern, m.Language.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("could not get all types for generation: %w", err)
	}

	for i := range types {
		types[i].model = m
	}

	return types, nil
}

func (m *Model) pokemonTypeCombo(ctx context.Context, pokemon *Pokemon) (*TypeCombo, error) {
	if m.Version == nil {
		return nil, ErrUnsetVersion
	}

	g, err := m.latestGeneration(ctx)
	if err != nil {
		return nil, fmt.Errorf("error while getting latest generation: %w", err)
	}

	gen, err := m.Version.Generation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get generation for model version: %w", err)
	}

	var ids []struct {
		ID int `db:"id"`
	}
	err = m.db.SelectContext(ctx, &ids,
		/* sql */ `
		SELECT DISTINCT FIRST_VALUE(type_id) OVER (
			PARTITION BY slot
			ORDER BY generation_id ASC
		) AS id
		FROM (
			SELECT type_id, pokemon_id, slot, ? AS generation_id
			FROM pokemon_v2_pokemontype
			UNION ALL
			SELECT type_id, pokemon_id, slot, generation_id
			FROM pokemon_v2_pokemontypepast
		)
		WHERE pokemon_id = ? AND generation_id >= ?
	`, g.ID, pokemon.ID, gen.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get type ids for pokemon %q: %w", pokemon.Name, err)
	}

	t1, err := m.typeByID(ctx, ids[0].ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get first type for pokemon %q: %w", pokemon.Name, err)
	}

	var t2 *Type
	if len(ids) > 1 {
		t2, err = m.typeByID(ctx, ids[1].ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get second type for pokemon %q: %w", pokemon.Name, err)
		}
	}

	return &TypeCombo{
		model: m,
		Type1: t1,
		Type2: t2,
	}, nil
}
