package data

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID          uuid.UUID `db:"id"`
	Name        string    `db:"name"`
	DisplayName string    `db:"display_name"`
	Email       string    `db:"email"`
	Avatar      string    `db:"avatar"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

type Keyword struct {
	ID        int       `db:"id"`
	UserID    uuid.UUID `db:"user_id"`
	Keyword   string    `db:"keyword"`
	Active    bool      `db:"active"`
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

type Match struct {
	ID         int64          `db:"id"`
	UserID     uuid.UUID      `db:"user_id"`
	KeywordID  sql.NullInt64  `db:"keyword_id"`
	Source     string         `db:"source"`
	URL        sql.NullString `db:"url"`
	MatchHash  string         `db:"match_hash"`
	NotifiedAt sql.NullTime   `db:"notified_at"`
	Data       []byte         `db:"data"`
	CreatedAt  time.Time      `db:"created_at"`
}
