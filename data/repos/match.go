package repos

import (
	"database/sql"
	"encoding/json"
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
		INSERT INTO matches (user_id, keyword_id, source, url, match_hash, data, created_at)
		VALUES (:user_id, :keyword_id, :source, :url, :match_hash, :data, :created_at)
		ON CONFLICT (match_hash) DO NOTHING`

	_, err := r.db.NamedExec(query, matches)
	if err != nil {
		return fmt.Errorf("create matches: %w", err)
	}

	return nil
}

func (r *MatchRepo) GetUnnotifiedMatches() ([]data.Match, error) {
	var matches []data.Match
	query := `
		SELECT id, user_id, keyword_id, source, url, match_hash, notified_at, data, created_at
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

	query, args, err := sqlx.In(
		`UPDATE matches SET notified_at = ? WHERE id IN (?)`,
		notifiedAt, ids,
	)
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

type MatchPayload struct {
	Keyword   string `json:"keyword"`
	Subreddit string `json:"subreddit,omitempty"`
	Author    string `json:"author,omitempty"`
	Title     string `json:"title,omitempty"`
	Body      string `json:"body,omitempty"`
	IsComment bool   `json:"is_comment,omitempty"`
}

func BuildMatchPayload(match data.Match) (MatchPayload, error) {
	var payload MatchPayload
	if len(match.Data) == 0 {
		return payload, nil
	}

	if err := json.Unmarshal(match.Data, &payload); err != nil {
		return payload, fmt.Errorf("unmarshal match payload: %w", err)
	}

	return payload, nil
}

func NewMatch(userID uuid.UUID, keywordID sql.NullInt64, source string, url sql.NullString, matchHash string, payload MatchPayload) (data.Match, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return data.Match{}, fmt.Errorf("marshal match payload: %w", err)
	}

	return data.Match{
		UserID:    userID,
		KeywordID: keywordID,
		Source:    source,
		URL:       url,
		MatchHash: matchHash,
		Data:      raw,
		CreatedAt: time.Now(),
	}, nil
}
