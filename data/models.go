package data

type KeywordNotification struct {
	Keyword string `db:"keyword"`
	Email   string `db:"email"`
}
