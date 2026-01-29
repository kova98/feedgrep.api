package data

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/kova98/feedgrep.api/enums"
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
	ID         int             `db:"id"`
	UserID     uuid.UUID       `db:"user_id"`
	KeywordID  int             `db:"keyword_id"`
	Source     enums.Source    `db:"source"`
	Hash       string          `db:"hash"`
	DataRaw    json.RawMessage `db:"data"`
	NotifiedAt time.Time       `db:"notified_at"`
	CreatedAt  time.Time       `db:"created_at"`
}

func NewMatch(userID uuid.UUID, keywordID int, source enums.Source, hash string, data any) (Match, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Match{}, err
	}

	return Match{
		UserID:    userID,
		KeywordID: keywordID,
		Source:    source,
		Hash:      hash,
		DataRaw:   raw,
	}, nil
}

type RedditData struct {
	Keyword   string `json:"keyword"`
	Subreddit string `json:"subreddit"`
	Author    string `json:"author"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Permalink string `json:"permalink"`
	IsComment bool   `json:"is_comment"`
}
