package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var errPlayerResolutionChanged = errors.New("player name changed during resolution")

type tournamentProfile struct {
	id               uuid.UUID
	name, normalized string
	deleted          bool
}

type tournamentQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func loadTournamentProfiles(ctx context.Context, db tournamentQuerier, owner uuid.UUID, ids []uuid.UUID, lock bool) (map[uuid.UUID]tournamentProfile, error) {
	profiles := make(map[uuid.UUID]tournamentProfile, len(ids))
	if len(ids) == 0 {
		return profiles, nil
	}
	query := `SELECT id, name, normalized_name, deleted_at IS NOT NULL FROM players
		WHERE user_id = $1 AND id = ANY($2::uuid[]) ORDER BY id`
	if lock {
		query += " FOR SHARE"
	}
	rows, err := db.Query(ctx, query, owner, ids)
	if err != nil {
		return nil, fmt.Errorf("load tournament players: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var profile tournamentProfile
		if err := rows.Scan(&profile.id, &profile.name, &profile.normalized, &profile.deleted); err != nil {
			return nil, fmt.Errorf("scan tournament player: %w", err)
		}
		profiles[profile.id] = profile
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tournament players: %w", err)
	}
	for _, id := range ids {
		if _, ok := profiles[id]; !ok {
			return nil, errors.Join(app.ErrPlayerNotFound, domain.ErrPlayerNotFound)
		}
	}
	return profiles, nil
}

func hydrateTournamentPlayers(cards []*domain.Tournament, profiles map[uuid.UUID]tournamentProfile) error {
	for _, card := range cards {
		for i := range card.Players {
			profile, ok := profiles[card.Players[i].ID]
			if !ok {
				return errors.Join(app.ErrPlayerNotFound, domain.ErrPlayerNotFound)
			}
			card.Players[i].Name, card.Players[i].IsDeleted = profile.name, profile.deleted
		}
	}
	return nil
}

func tournamentPlayerIDs(cards []*domain.Tournament) []uuid.UUID {
	seen := make(map[uuid.UUID]bool)
	for _, card := range cards {
		for _, player := range card.Players {
			seen[player.ID] = true
		}
	}
	ids := make([]uuid.UUID, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return ids
}

func resolveTournamentPlayers(ctx context.Context, tx pgx.Tx, owner uuid.UUID, selections []domain.TournamentPlayer, original *domain.Tournament) ([]domain.TournamentPlayer, error) {
	keys := make([]string, 0)
	displays := make(map[string]string)
	selectionKeys := make([]string, len(selections))
	for i, selection := range selections {
		if selection.ID != uuid.Nil {
			continue
		}
		display, key, err := domain.NormalizePlayerName(selection.Name)
		if err != nil {
			return nil, errors.Join(app.ErrValidation, err)
		}
		selectionKeys[i] = key
		if _, ok := displays[key]; !ok {
			keys = append(keys, key)
			displays[key] = display
		}
	}
	slices.Sort(keys)
	byName := make(map[string]uuid.UUID)
	if len(keys) > 0 {
		rows, err := tx.Query(ctx, `SELECT id, normalized_name FROM players
			WHERE user_id = $1 AND deleted_at IS NULL AND normalized_name = ANY($2::text[])`, owner, keys)
		if err != nil {
			return nil, fmt.Errorf("match tournament players: %w", err)
		}
		for rows.Next() {
			var id uuid.UUID
			var key string
			if err := rows.Scan(&id, &key); err != nil {
				rows.Close()
				return nil, fmt.Errorf("scan player match: %w", err)
			}
			byName[key] = id
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, fmt.Errorf("iterate player matches: %w", err)
		}
	}
	oldIDs := make(map[uuid.UUID]bool)
	known := &domain.Tournament{Players: slices.Clone(selections)}
	for i := range known.Players {
		if known.Players[i].ID == uuid.Nil {
			known.Players[i].ID = byName[selectionKeys[i]]
		}
	}
	cards := []*domain.Tournament{known}
	if original != nil {
		cards = append(cards, original)
		for _, player := range original.Players {
			oldIDs[player.ID] = true
		}
	}
	ids := tournamentPlayerIDs(cards)
	ids = slices.DeleteFunc(ids, func(id uuid.UUID) bool { return id == uuid.Nil })
	profiles, err := loadTournamentProfiles(ctx, tx, owner, ids, true)
	if err != nil {
		return nil, err
	}
	for key, id := range byName {
		profile := profiles[id]
		if profile.deleted || profile.normalized != key {
			return nil, errPlayerResolutionChanged
		}
	}
	if original != nil {
		if err := hydrateTournamentPlayers([]*domain.Tournament{original}, profiles); err != nil {
			return nil, err
		}
	}
	for _, key := range keys {
		if _, ok := byName[key]; ok {
			continue
		}
		id := uuid.New()
		err := tx.QueryRow(ctx, `INSERT INTO players (id, user_id, name, normalized_name, created_at, updated_at)
			VALUES ($1, $2, $3, $4, NOW(), NOW())
			ON CONFLICT (user_id, normalized_name) WHERE deleted_at IS NULL DO NOTHING RETURNING id`, id, owner, displays[key], key).Scan(&id)
		profile := tournamentProfile{id: id, name: displays[key], normalized: key}
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT id, name, normalized_name, deleted_at IS NOT NULL FROM players
				WHERE user_id = $1 AND normalized_name = $2 AND deleted_at IS NULL FOR SHARE`, owner, key).Scan(&profile.id, &profile.name, &profile.normalized, &profile.deleted)
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, errPlayerResolutionChanged
			}
		}
		if err != nil {
			return nil, fmt.Errorf("create tournament player: %w", err)
		}
		byName[key] = profile.id
		profiles[profile.id] = profile
	}
	resolved := slices.Clone(selections)
	for i, selection := range selections {
		id := selection.ID
		if id == uuid.Nil {
			id = byName[selectionKeys[i]]
		}
		profile := profiles[id]
		if profile.deleted && !oldIDs[id] {
			return nil, errors.Join(app.ErrPlayerNotFound, domain.ErrPlayerNotFound)
		}
		resolved[i].ID, resolved[i].Name, resolved[i].IsDeleted = id, profile.name, profile.deleted
		resolved[i].Rebuys = slices.Clone(selection.Rebuys)
	}
	return resolved, nil
}

func rollbackTournamentTx(ctx context.Context, tx pgx.Tx) {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = tx.Rollback(cleanup)
}
