package repos

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/kova98/feedgrep.api/data"
)

type KeywordRepo struct {
	db *sqlx.DB
}

func NewKeywordRepo(db *sqlx.DB) *KeywordRepo {
	return &KeywordRepo{db}
}

func (r *KeywordRepo) CreateKeyword(k data.Keyword) (int, error) {
	filtersRaw, err := json.Marshal(k.Filters)
	if err != nil {
		return 0, errors.Wrap(err, "marshal filters: ")
	}
	k.FiltersRaw = filtersRaw

	query := `
		INSERT INTO keywords (user_id, keyword, filters)
		VALUES (:user_id, :keyword, :filters)
		ON CONFLICT (user_id, LOWER(keyword)) DO UPDATE 
		    SET active = EXCLUDED.active, 
		        filters = EXCLUDED.filters, 
		        updated_at = now()
		RETURNING id`

	rows, err := r.db.NamedQuery(query, k)
	if err != nil {
		return 0, fmt.Errorf("create keyword: %w", err)
	}
	defer rows.Close()

	var id int
	if rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("scan returned id: %w", err)
		}
		return id, nil
	}

	return id, nil
}

func (r *KeywordRepo) GetKeywordsByUserID(userID uuid.UUID) ([]data.Keyword, error) {
	var keywords []data.Keyword
	query := `
		SELECT k.id, k.user_id, k.keyword, k.active, k.filters, k.created_at, k.updated_at,
		       COUNT(m.id) AS hit_count
		FROM keywords k
		LEFT JOIN matches m ON m.keyword_id = k.id
		WHERE k.user_id = $1
		GROUP BY k.id
		ORDER BY k.created_at DESC`

	err := r.db.Select(&keywords, query, userID)
	if err != nil {
		return nil, fmt.Errorf("get keywords by user id: %w", err)
	}

	for i := range keywords {
		if err := json.Unmarshal(keywords[i].FiltersRaw, &keywords[i].Filters); err != nil {
			return nil, fmt.Errorf("unmarshal filters %d: %w", keywords[i].ID, err)
		}
	}

	return keywords, nil
}

func (r *KeywordRepo) GetKeywordByID(id int, userID uuid.UUID) (*data.Keyword, error) {
	var keyword data.Keyword
	query := "SELECT * FROM keywords WHERE id = $1 AND user_id = $2"

	err := r.db.Get(&keyword, query, id, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get keyword by id: %w", err)
	}

	if err := json.Unmarshal(keyword.FiltersRaw, &keyword.Filters); err != nil {
		return nil, fmt.Errorf("unmarshal filters %d: %w", keyword.ID, err)
	}

	return &keyword, nil
}

func (r *KeywordRepo) GetActiveKeywords() ([]data.Keyword, error) {
	var keywords []data.Keyword
	query := `
		SELECT id, user_id, keyword, active, filters, created_at, updated_at
		FROM keywords
		WHERE active = true
		ORDER BY created_at DESC`

	err := r.db.Select(&keywords, query)
	if err != nil {
		return nil, fmt.Errorf("get active keywords: %w", err)
	}

	for i := range keywords {
		if err := json.Unmarshal(keywords[i].FiltersRaw, &keywords[i].Filters); err != nil {
			return nil, fmt.Errorf("unmarshal filters %d: %w", keywords[i].ID, err)
		}
	}

	return keywords, nil
}

func (r *KeywordRepo) GetActiveKeywordsWithEmails() ([]data.KeywordNotification, error) {
	var keywords []data.KeywordNotification
	query := `
		SELECT k.id, k.user_id, k.keyword, k.filters, u.email
		FROM keywords k
		JOIN users u ON u.id = k.user_id
		WHERE k.active = true
		ORDER BY k.created_at DESC`

	err := r.db.Select(&keywords, query)
	if err != nil {
		return nil, fmt.Errorf("get active keywords with emails: %w", err)
	}

	for i := range keywords {
		if err := json.Unmarshal(keywords[i].FiltersRaw, &keywords[i].Filters); err != nil {
			return nil, fmt.Errorf("unmarshal filters %d: %w", keywords[i].ID, err)
		}
	}

	return keywords, nil
}

func (r *KeywordRepo) UpdateKeyword(k data.Keyword) error {
	filtersRaw, err := json.Marshal(k.Filters)
	if err != nil {
		return errors.Wrap(err, "marshal filters: ")
	}
	k.FiltersRaw = filtersRaw

	query := `
		UPDATE keywords
		SET keyword = :keyword, active = :active, filters = :filters, updated_at = now()
		WHERE id = :id AND user_id = :user_id`

	rows, err := r.db.NamedQuery(query, k)
	if err != nil {
		return fmt.Errorf("update keyword: %w", err)
	}
	defer rows.Close()

	return nil
}

func (r *KeywordRepo) DeleteKeyword(id int, userID uuid.UUID) error {
	query := "DELETE FROM keywords WHERE id = $1 AND user_id = $2"
	_, err := r.db.Exec(query, id, userID)
	if err != nil {
		return fmt.Errorf("delete keyword: %w", err)
	}

	return nil
}
