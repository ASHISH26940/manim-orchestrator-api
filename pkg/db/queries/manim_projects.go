package queries

import (
	"database/sql"
	"time" // For HTTP client timeout

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)



// CreateManimProject inserts a new Manim project into the database.
// It now includes 'prompt' and 'render_status' in the insert.
func CreateManimProject(project *db.ManimProject) (*db.ManimProject, error) {
	// Ensure default status if not set
	if project.RenderStatus == "" {
		project.RenderStatus = "pending"
	}

	query := `
		INSERT INTO manim_projects (user_id, name, description, prompt, render_status)
		VALUES (:user_id, :name, :description, :prompt, :render_status)
		RETURNING id, created_at, updated_at`

	rows, err := db.DB.NamedQuery(query, project)
	if err != nil {
		log.Errorf("Error creating Manim project: %v", err)
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		err := rows.StructScan(project)
		if err != nil {
			log.Errorf("Error scanning Manim project data after creation: %v", err)
			return nil, err
		}
	} else {
		log.Error("No rows returned after Manim project creation.")
		return nil, nil
	}

	log.Infof("Manim project '%s' created for user ID: %s (ID: %s)", project.Name, project.UserID.String(), project.ID.String())
	return project, nil
}

// FindManimProjectByID retrieves a Manim project by its ID.
// Includes new fields in the SELECT.
func FindManimProjectByID(projectID uuid.UUID) (*db.ManimProject, error) {
	project := &db.ManimProject{}
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at FROM manim_projects WHERE id = $1`
	err := db.DB.Get(project, query, projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("Manim project with ID '%s' not found.", projectID.String())
			return nil, nil
		}
		log.Errorf("Error finding Manim project by ID '%s': %v", projectID.String(), err)
		return nil, err
	}
	return project, nil
}

// FindManimProjectsByUserID retrieves all Manim projects for a specific user ID.
// Includes new fields in the SELECT.
func FindManimProjectsByUserID(userID uuid.UUID) ([]db.ManimProject, error) {
	var projects []db.ManimProject
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at FROM manim_projects WHERE user_id = $1 ORDER BY created_at DESC`
	err := db.DB.Select(&projects, query, userID)
	if err != nil {
		log.Errorf("Error finding Manim projects for user ID '%s': %v", userID.String(), err)
		return nil, err
	}
	return projects, nil
}

// FindManimProjectByNameAndUserID retrieves a Manim project by its name and user ID.
// Includes new fields in the SELECT.
func FindManimProjectByNameAndUserID(name string, userID uuid.UUID) (*db.ManimProject, error) {
    project := &db.ManimProject{}
    query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at FROM manim_projects WHERE name = $1 AND user_id = $2`
    err := db.DB.Get(project, query, name, userID)
    if err != nil {
        if err == sql.ErrNoRows {
            log.Debugf("Manim project with name '%s' not found for user ID '%s'.", name, userID.String())
            return nil, nil
        }
        log.Errorf("Error finding Manim project by name '%s' for user ID '%s': %v", name, userID.String(), err)
        return nil, err
    }
    return project, nil
}

// UpdateManimProject updates an existing Manim project in the database.
// Includes new fields in the UPDATE.
func UpdateManimProject(project *db.ManimProject) error {
	project.UpdatedAt = time.Now().UTC()

	query := `
		UPDATE manim_projects
		SET name = :name, description = :description, prompt = :prompt, render_status = :render_status, video_url = :video_url, updated_at = :updated_at
		WHERE id = :id AND user_id = :user_id`

	result, err := db.DB.NamedExec(query, project)
	if err != nil {
		log.Errorf("Error updating Manim project with ID '%s': %v", project.ID.String(), err)
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warnf("No Manim project found with ID '%s' for user ID '%s' for update.", project.ID.String(), project.UserID.String())
		return sql.ErrNoRows
	}

	log.Infof("Manim project with ID '%s' updated.", project.ID.String())
	return nil
}

// DeleteManimProject (no changes needed here as it deletes by ID)
func DeleteManimProject(projectID, userID uuid.UUID) error {
    query := `DELETE FROM manim_projects WHERE id = $1 AND user_id = $2`
    result, err := db.DB.Exec(query, projectID, userID)
    if err != nil {
        log.Errorf("Error deleting Manim project with ID '%s' for user ID '%s': %v", projectID.String(), userID.String(), err)
        return err
    }

    rowsAffected, _ := result.RowsAffected()
    if rowsAffected == 0 {
        log.Warnf("No Manim project found with ID '%s' for user ID '%s' for deletion.", projectID.String(), userID.String())
        return sql.ErrNoRows
    }

    log.Infof("Manim project with ID '%s' deleted.", projectID.String())
    return nil
}

