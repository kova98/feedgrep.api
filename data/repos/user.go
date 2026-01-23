package repos

import (
	"github.com/kova98/feedgrep.api/data"
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type UserRepo struct {
	db *sqlx.DB
}

func NewUserRepo(db *sqlx.DB) *UserRepo {
	return &UserRepo{db}
}

func (r UserRepo) InsertUser(user data.User) (uuid.UUID, error) {
	query := `
		INSERT INTO users (id, name, email, avatar) 
		VALUES (:id, :name, :email, :avatar)
		RETURNING id`

	rows, err := r.db.NamedQuery(query, user)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert user: %w", err)
	}
	defer rows.Close()

	var id uuid.UUID
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return uuid.Nil, fmt.Errorf("scan returned id: %w", err)
		}
	}

	return id, nil
}

func (r UserRepo) GetUserByID(id uuid.UUID) (*data.User, error) {
	var user data.User
	query := "SELECT * FROM users WHERE id = $1"
	err := r.db.Get(&user, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return &user, nil
}
