package data

import (
	"encoding/json"

	"github.com/google/uuid"
)

type KeywordNotification struct {
	ID         int             `db:"id"`
	UserID     uuid.UUID       `db:"user_id"`
	Keyword    string          `db:"keyword"`
	Email      string          `db:"email"`
	FiltersRaw json.RawMessage `db:"filters"`
	Filters    KeywordFilters  `db:"-"`
}

type MatchWithKeyword struct {
	Match
	Keyword string `db:"keyword"`
}
