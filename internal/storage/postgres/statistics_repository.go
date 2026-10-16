package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"holdem-tournament-builder/internal/app"
	"holdem-tournament-builder/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type statisticsDB interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

type StatisticsRepository struct{ pool statisticsDB }

func NewStatisticsRepository(pool *pgxpool.Pool) *StatisticsRepository {
	return &StatisticsRepository{pool: pool}
}

func (r *StatisticsRepository) Leaderboard(ctx context.Context, owner uuid.UUID, limit, offset int) ([]domain.PlayerStatistics, error) {
	if owner == uuid.Nil {
		return nil, app.ErrUnauthenticated
	}
	rows, err := r.pool.Query(ctx, `
		SELECT p.id,
		       p.name,
		       COUNT(*)::bigint AS tournaments_played,
		       SUM(tp.contribution)::text AS total_buy_ins,
		       SUM(tp.prize)::text AS total_prizes,
		       (SUM(tp.prize) - SUM(tp.contribution))::text AS net_profit
		FROM tournament_players tp
		JOIN players p ON p.id = tp.player_id AND p.user_id = tp.user_id
		WHERE tp.user_id = $1
		  AND tp.included_in_statistics
		  AND p.deleted_at IS NULL
		GROUP BY p.id, p.name, p.normalized_name
		ORDER BY SUM(tp.prize) - SUM(tp.contribution) DESC,
		         SUM(tp.prize) DESC,
		         p.normalized_name ASC,
		         p.id ASC
		LIMIT $2 OFFSET $3
	`, owner, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("query player statistics: %w", err)
	}
	defer rows.Close()

	items := make([]domain.PlayerStatistics, 0)
	for rows.Next() {
		var item domain.PlayerStatistics
		var totalBuyIns, totalPrizes, netProfit string
		if err := rows.Scan(
			&item.PlayerID,
			&item.Name,
			&item.TournamentsPlayed,
			&totalBuyIns,
			&totalPrizes,
			&netProfit,
		); err != nil {
			return nil, fmt.Errorf("scan player statistics: %w", err)
		}
		var ok bool
		if item.TotalBuyIns, ok = new(big.Int).SetString(totalBuyIns, 10); !ok {
			return nil, errors.New("parse player statistics: invalid total buy-ins")
		}
		if item.TotalPrizes, ok = new(big.Int).SetString(totalPrizes, 10); !ok {
			return nil, errors.New("parse player statistics: invalid total prizes")
		}
		if item.NetProfit, ok = new(big.Int).SetString(netProfit, 10); !ok {
			return nil, errors.New("parse player statistics: invalid net profit")
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate player statistics: %w", err)
	}
	return items, nil
}
