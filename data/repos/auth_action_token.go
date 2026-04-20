package repos

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
)

type AuthActionTokenRepo struct {
	db *sqlx.DB
}

func NewAuthActionTokenRepo(db *sqlx.DB) *AuthActionTokenRepo {
	return &AuthActionTokenRepo{db: db}
}

func (r *AuthActionTokenRepo) Insert(token data.AuthActionToken) error {
	query := `
		INSERT INTO auth_action_tokens (id, user_id, email, action, token_hash, expires_at)
		VALUES (:id, :user_id, :email, :action, :token_hash, :expires_at)`

	if _, err := r.db.NamedExec(query, token); err != nil {
		return fmt.Errorf("insert auth action token: %w", err)
	}

	return nil
}

func (r *AuthActionTokenRepo) GetValid(action, tokenHash string, now time.Time) (*data.AuthActionToken, error) {
	var token data.AuthActionToken
	query := `
		SELECT id, user_id, email, action, token_hash, expires_at, used_at, created_at
		FROM auth_action_tokens
		WHERE action = $1
		  AND token_hash = $2
		  AND used_at IS NULL
		  AND expires_at > $3`

	err := r.db.Get(&token, query, action, tokenHash, now)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get valid auth action token: %w", err)
	}

	return &token, nil
}

func (r *AuthActionTokenRepo) MarkUsed(id uuid.UUID) error {
	query := `
		UPDATE auth_action_tokens
		SET used_at = now()
		WHERE id = $1`

	if _, err := r.db.Exec(query, id); err != nil {
		return fmt.Errorf("mark auth action token used: %w", err)
	}

	return nil
}
