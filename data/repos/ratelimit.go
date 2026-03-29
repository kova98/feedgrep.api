package repos

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
)

type RateLimitRepo struct {
	db *sqlx.DB
}

func NewRateLimitRepo(db *sqlx.DB) *RateLimitRepo {
	return &RateLimitRepo{db: db}
}

func (r *RateLimitRepo) GetCounter(userID uuid.UUID, rateID, windowKey string) (*data.RateLimitCounter, error) {
	var counter data.RateLimitCounter
	query := `
		SELECT user_id, rate_id, window_key, count, created_at, updated_at
		FROM rate_limits
		WHERE user_id = $1 AND rate_id = $2 AND window_key = $3`

	err := r.db.Get(&counter, query, userID, rateID, windowKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get rate limit counter: %w", err)
	}

	return &counter, nil
}

func (r *RateLimitRepo) IncrementWithinLimit(userID uuid.UUID, rateID, windowKey string, limit int) (int, bool, error) {
	query := `
		INSERT INTO rate_limits (user_id, rate_id, window_key, count, created_at, updated_at)
		VALUES ($1, $2, $3, 1, now(), now())
		ON CONFLICT (user_id, rate_id, window_key)
		DO UPDATE
		SET count = rate_limits.count + 1,
		    updated_at = now()
		WHERE rate_limits.count < $4
		RETURNING count`

	var count int
	err := r.db.Get(&count, query, userID, rateID, windowKey, limit)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return limit, false, nil
		}
		return 0, false, fmt.Errorf("increment rate limit counter: %w", err)
	}

	return count, true, nil
}
