package repos

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
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

func (r UserRepo) GetUsersByIDs(IDs []uuid.UUID) ([]data.User, error) {
	if len(IDs) == 0 {
		return []data.User{}, nil
	}

	var users []data.User
	query, args, err := sqlx.In(`
		SELECT id, name, email, avatar, created_at, updated_at
		FROM users
		WHERE id IN (?)`, IDs)
	query = r.db.Rebind(query)
	if err != nil {
		return nil, fmt.Errorf("build get users by ids: %w", err)
	}

	err = r.db.Select(&users, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []data.User{}, nil
		}
		return nil, fmt.Errorf("get users by ids: %w", err)
	}

	return users, nil
}
