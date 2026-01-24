package models

import "github.com/google/uuid"

type CreateKeywordRequest struct {
	Keyword string `json:"keyword"`
}

type UpdateKeywordRequest struct {
	Keyword string `json:"keyword"`
	Active  bool   `json:"active"`
}

type Keyword struct {
	ID      int       `json:"id"`
	UserID  uuid.UUID `json:"userId"`
	Keyword string    `json:"keyword"`
	Active  bool      `json:"active"`
}

type GetKeywordsResponse struct {
	Keywords []Keyword `json:"keywords"`
}
