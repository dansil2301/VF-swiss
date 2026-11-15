package models

import "time"

type Request struct {
	ID          int64     `db:"id"`
	Name        string    `db:"name"`
	Phone       string    `db:"phone"`
	Email       string    `db:"email"`
	Brand       string    `db:"brand"`
	Description string    `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
}

type RequestPhoto struct {
	ID           int64     `db:"id"`
	RequestID    int64     `db:"request_id"`
	Filename     string    `db:"filename"`
	OriginalName string    `db:"original_name"`
	MIMEType     string    `db:"mime_type"`
	Size         int64     `db:"size"`
	CreatedAt    time.Time `db:"created_at"`
}
