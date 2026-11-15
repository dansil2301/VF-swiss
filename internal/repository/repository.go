package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vfswiss/vfswiss/internal/models"
)

type Repository struct {
	db *sql.DB
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) CreateRequest(ctx context.Context, request *models.Request) error {
	stmt := `INSERT INTO requests (name, phone, email, brand, description, created_at) VALUES (?, ?, ?, ?, ?, ?);`
	result, err := r.db.ExecContext(ctx, stmt,
		request.Name,
		request.Phone,
		request.Email,
		request.Brand,
		request.Description,
		request.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create request id: %w", err)
	}
	request.ID = id
	return nil
}

func (r *Repository) CreateRequestPhoto(ctx context.Context, photo *models.RequestPhoto) error {
	stmt := `INSERT INTO request_photos (request_id, filename, original_name, mime_type, size, created_at) VALUES (?, ?, ?, ?, ?, ?);`
	result, err := r.db.ExecContext(ctx, stmt,
		photo.RequestID,
		photo.Filename,
		photo.OriginalName,
		photo.MIMEType,
		photo.Size,
		photo.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("create request photo: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("create request photo id: %w", err)
	}
	photo.ID = id
	return nil
}

func (r *Repository) GetRequest(ctx context.Context, id int64) (*models.Request, error) {
	stmt := `SELECT id, name, phone, email, brand, description, created_at FROM requests WHERE id = ?;`
	row := r.db.QueryRowContext(ctx, stmt, id)
	req := &models.Request{}
	if err := row.Scan(&req.ID, &req.Name, &req.Phone, &req.Email, &req.Brand, &req.Description, &req.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get request: %w", err)
	}
	return req, nil
}

func (r *Repository) GetRequestPhotos(ctx context.Context, requestID int64) ([]*models.RequestPhoto, error) {
	stmt := `SELECT id, request_id, filename, original_name, mime_type, size, created_at FROM request_photos WHERE request_id = ?;`
	rows, err := r.db.QueryContext(ctx, stmt, requestID)
	if err != nil {
		return nil, fmt.Errorf("get request photos: %w", err)
	}
	defer rows.Close()

	var photos []*models.RequestPhoto
	for rows.Next() {
		photo := &models.RequestPhoto{}
		if err := rows.Scan(&photo.ID, &photo.RequestID, &photo.Filename, &photo.OriginalName, &photo.MIMEType, &photo.Size, &photo.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan request photo: %w", err)
		}
		photos = append(photos, photo)
	}
	return photos, nil
}

func (r *Repository) DB() *sql.DB {
	return r.db
}
