package data

import "github.com/google/uuid"

type KeywordNotification struct {
	ID      int       `db:"id"`
	UserID  uuid.UUID `db:"user_id"`
	Keyword string    `db:"keyword"`
	Email   string    `db:"email"`
}
