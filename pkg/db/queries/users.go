package queries

import (
	"time"
	"database/sql"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db" // Import your db package
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// CreateUser inserts a new user into the database.
// It takes a User struct (without ID, CreatedAt, UpdatedAt) and returns the created User with generated fields.
func CreateUser(user *db.User) (*db.User, error) {
	// Ensure timestamps are set before insertion if they aren't by the DB default.
	// However, PostgreSQL's DEFAULT CURRENT_TIMESTAMP handles this well.
	// We might use NOW() in the query for more explicit control or if DB default is not set.

	query := `
		INSERT INTO users (username, email, password_hash)
		VALUES (:username, :email, :password_hash)
		RETURNING id, created_at, updated_at` // RETURNING allows us to get generated fields

	// Use NamedExec for queries with named parameters from struct tags.
	// This executes the query and returns the first row's generated fields into 'user'.
	rows, err := db.DB.NamedQuery(query, user)
	if err != nil {
		log.Errorf("Error creating user: %v", err)
		return nil, err
	}
	defer rows.Close() // Always close rows after use

	// Scan the returned values back into the user struct.
	if rows.Next() {
		err := rows.StructScan(user)
		if err != nil {
			log.Errorf("Error scanning user data after creation: %v", err)
			return nil, err
		}
	} else {
		log.Error("No rows returned after user creation.")
		return nil, nil // Or an appropriate error
	}

	log.Infof("User %s created with ID: %s", user.Email, user.ID.String())
	return user, nil
}

// FindUserByEmail retrieves a user from the database by their email address.
func FindUserByEmail(email string) (*db.User, error) {
	user := &db.User{}
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE email = $1`
	err := db.DB.Get(user, query, email) // Get is for single row results
	if err != nil {
		// sql.ErrNoRows is a common error to check for when a record isn't found
		if err == sql.ErrNoRows {
			log.Debugf("User with email '%s' not found.", email)
			return nil, nil // Return nil, nil to indicate not found without an error
		}
		log.Errorf("Error finding user by email '%s': %v", email, err)
		return nil, err
	}
	return user, nil
}

// FindUserByID retrieves a user from the database by their ID.
func FindUserByID(id uuid.UUID) (*db.User, error) {
	user := &db.User{}
	query := `SELECT id, username, email, password_hash, created_at, updated_at FROM users WHERE id = $1`
	err := db.DB.Get(user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("User with ID '%s' not found.", id.String())
			return nil, nil
		}
		log.Errorf("Error finding user by ID '%s': %v", id.String(), err)
		return nil, err
	}
	return user, nil
}

// UpdateUser updates an existing user's information in the database.
// It expects the user struct to have the ID set for the record to update.
func UpdateUser(user *db.User) error {
	user.UpdatedAt = time.Now().UTC() // Update the timestamp manually before saving

	query := `
		UPDATE users
		SET username = :username, email = :email, password_hash = :password_hash, updated_at = :updated_at
		WHERE id = :id`

	result, err := db.DB.NamedExec(query, user)
	if err != nil {
		log.Errorf("Error updating user with ID '%s': %v", user.ID.String(), err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warnf("No user found with ID '%s' for update.", user.ID.String())
		return nil // Or return a specific "not found" error if desired
	}

	log.Infof("User with ID '%s' updated.", user.ID.String())
	return nil
}

// DeleteUser deletes a user from the database by their ID.
func DeleteUser(id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`
	result, err := db.DB.Exec(query, id) // Exec is for queries that don't return rows (INSERT, UPDATE, DELETE)
	if err != nil {
		log.Errorf("Error deleting user with ID '%s': %v", id.String(), err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warnf("No user found with ID '%s' for deletion.", id.String())
		return nil // Or return a specific "not found" error
	}

	log.Infof("User with ID '%s' deleted.", id.String())
	return nil
}