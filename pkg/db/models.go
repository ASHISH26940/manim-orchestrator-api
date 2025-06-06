package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `db:"id"`            // primary key, auto-generated UUID
	Username     string    `db:"username"`      // unique username
	Email        string    `db:"email"`         // unique email
	PasswordHash string    `db:"password_hash"` // hashed password
	CreatedAt    time.Time `db:"created_at"`    // timestamp of creation
	UpdatedAt    time.Time `db:"updated_at"`    // timestamp of last update
}

type ManimProject struct {
	ID          uuid.UUID `db:"id"`
	UserID      uuid.UUID `db:"user_id"`
	Name        string    `db:"name"`
	Description string    `db:"description"`
    Prompt      string    `db:"prompt"`       // <--- NEW FIELD
    RenderStatus string   `db:"render_status"`// <--- NEW FIELD (e.g., "pending", "rendering", "completed", "failed")
    VideoURL    sql.NullString    `db:"video_url"`    // <--- NEW FIELD (URL of the final video)
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}