package repos

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
)

type MatchRepo struct {
	db *sqlx.DB
}

func NewMatchRepo(db *sqlx.DB) *MatchRepo {
	return &MatchRepo{db}
}

func (r *MatchRepo) CreateMatches(matches []data.Match) error {
	if len(matches) == 0 {
		return nil
	}

	query := `
		INSERT INTO matches (user_id, keyword_id, source, hash, data, created_at)
		VALUES (:user_id, :keyword_id, :source, :hash, :data, now())
		ON CONFLICT (hash) DO NOTHING`

	_, err := r.db.NamedExec(query, matches)
	if err != nil {
		return fmt.Errorf("create matches: %w", err)
	}

	return nil
}

func (r *MatchRepo) GetUnnotifiedMatches() ([]data.Match, error) {
	var matches []data.Match
	query := `
		SELECT id, user_id, keyword_id, source, hash, notified_at, data, created_at
		FROM matches
		WHERE notified_at IS NULL
		ORDER BY created_at ASC`

	err := r.db.Select(&matches, query)
	if err != nil {
		return nil, fmt.Errorf("get unnotified matches: %w", err)
	}

	return matches, nil
}

func (r *MatchRepo) MarkNotified(ids []int64, notifiedAt time.Time) error {
	if len(ids) == 0 {
		return nil
	}

	query, args, err := sqlx.In(`UPDATE matches SET notified_at = ? WHERE id IN (?)`, notifiedAt, ids)
	if err != nil {
		return fmt.Errorf("build mark notified: %w", err)
	}
	query = r.db.Rebind(query)

	_, err = r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("mark notified: %w", err)
	}

	return nil
}

func (r *MatchRepo) GetMatchesByUserID(userID uuid.UUID, limit, offset int) ([]data.MatchWithKeyword, int, error) {
	var matches []data.MatchWithKeyword
	query := `
		SELECT m.id, m.user_id, m.keyword_id, m.source, m.hash, m.notified_at, m.data, m.created_at,
		       k.keyword
		FROM matches m
		LEFT JOIN keywords k ON k.id = m.keyword_id
		WHERE m.user_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2 OFFSET $3`

	err := r.db.Select(&matches, query, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("get matches by user id: %w", err)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM matches WHERE user_id = $1`
	err = r.db.Get(&total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count matches: %w", err)
	}

	return matches, total, nil
}
