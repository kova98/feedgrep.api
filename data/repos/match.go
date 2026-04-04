package repos

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
	"github.com/kova98/feedgrep.api/enums"
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
		INSERT INTO matches (user_id, keyword_id, source, hash, data, created_at, seen_at)
		VALUES (:user_id, :keyword_id, :source, :hash, :data, now(), NULL)
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
		SELECT id, user_id, keyword_id, source, hash, notified_at, seen_at, data, created_at
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
		SELECT m.id, m.user_id, m.keyword_id, m.source, m.hash, m.notified_at, m.seen_at, m.data, m.created_at,
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

func (r *MatchRepo) GetMatchesByKeyword(userID uuid.UUID, keywordID, limit int, unseenOnly bool) ([]data.MatchWithKeyword, error) {
	var matches []data.MatchWithKeyword
	query := `
		SELECT m.id, m.user_id, m.keyword_id, m.source, m.hash, m.notified_at, m.seen_at, m.data, m.created_at,
		       k.keyword
		FROM matches m
		LEFT JOIN keywords k ON k.id = m.keyword_id
		WHERE m.user_id = $1
		  AND m.keyword_id = $2
		  AND ($4 = false OR m.seen_at IS NULL)
		ORDER BY m.created_at DESC
		LIMIT $3`

	if err := r.db.Select(&matches, query, userID, keywordID, limit, unseenOnly); err != nil {
		return nil, fmt.Errorf("get matches by keyword: %w", err)
	}

	return matches, nil
}

func (r *MatchRepo) GetMatchedSubredditsByKeyword(userID uuid.UUID, keywordID, limit int) ([]data.MatchedSubredditSummary, error) {
	var rows []data.MatchedSubredditSummary
	query := `
		SELECT LOWER(m.data->>'subreddit') AS subreddit,
		       MAX(m.created_at) AS last_matched_at,
		       COUNT(*) AS match_count
		FROM matches m
		WHERE m.user_id = $1
		  AND m.keyword_id = $2
		  AND m.source = $3
		  AND m.data->>'subreddit' != ''
		GROUP BY LOWER(m.data->>'subreddit')
		ORDER BY MAX(m.created_at) DESC
		LIMIT $4`

	if err := r.db.Select(&rows, query, userID, keywordID, enums.SourceArcticShift, limit); err != nil {
		return nil, fmt.Errorf("get matched subreddits by keyword: %w", err)
	}

	return rows, nil
}

func (r *MatchRepo) UpdateSeen(userID uuid.UUID, matchID int, seen bool) (bool, error) {
	query := `
		UPDATE matches
		SET seen_at = CASE WHEN $3 THEN now() ELSE NULL END
		WHERE id = $1 AND user_id = $2`

	result, err := r.db.Exec(query, matchID, userID, seen)
	if err != nil {
		return false, fmt.Errorf("update match seen state: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("get rows affected for seen state update: %w", err)
	}
	if rowsAffected == 0 {
		return false, nil
	}

	return true, nil
}

func (r *MatchRepo) GetDailyMatchCountsByKeyword(userID uuid.UUID, keywordID, days int) ([]data.KeywordDailyMatchCountRow, error) {
	var rows []data.KeywordDailyMatchCountRow
	query := `
		WITH days AS (
			SELECT generate_series(
				(current_date - ($3::int - 1) * interval '1 day')::date,
				current_date::date,
				interval '1 day'
			)::date AS day
		)
		SELECT days.day AS day,
		       COALESCE(COUNT(m.id), 0) AS count
		FROM days
		LEFT JOIN matches m
		  ON m.user_id = $1
		 AND m.keyword_id = $2
		 AND (m.created_at AT TIME ZONE 'UTC')::date = days.day
		GROUP BY days.day
		ORDER BY days.day ASC`

	if err := r.db.Select(&rows, query, userID, keywordID, days); err != nil {
		return nil, fmt.Errorf("get daily match counts by keyword: %w", err)
	}

	return rows, nil
}
