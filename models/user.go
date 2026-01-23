package models

import (
	"github.com/google/uuid"
)

type UserModel struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email"`
	Avatar      string    `json:"avatar"`
}
