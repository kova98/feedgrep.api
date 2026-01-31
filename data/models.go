package data

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/enums"
)

type KeywordNotification struct {
	ID         int             `db:"id"`
	UserID     uuid.UUID       `db:"user_id"`
	Keyword    string          `db:"keyword"`
	MatchMode  enums.MatchMode `db:"match_mode"`
	Email      string          `db:"email"`
	FiltersRaw json.RawMessage `db:"filters"`
	Filters    KeywordFilters  `db:"-"`
}

type MatchWithKeyword struct {
	Match
	Keyword string `db:"keyword"`
}
