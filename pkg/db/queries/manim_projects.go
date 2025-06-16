// pkg/db/queries/project_queries.go

package queries

import (
	"database/sql"
	"fmt" // Import fmt for error formatting
	"time"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db" // Import your db package (assuming db.DB is *sqlx.DB)
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// CreateManimProject inserts a new Manim project into the database.
// It now includes 'prompt', 'render_status', 'video_url', and 'parent_project_id' in the insert.
func CreateManimProject(project *db.ManimProject) (*db.ManimProject, error) {
	// Ensure default status if not set
	if project.RenderStatus == "" {
		project.RenderStatus = "pending"
	}

	query := `
        INSERT INTO manim_projects (user_id, name, description, prompt, render_status, video_url, parent_project_id)
        VALUES (:user_id, :name, :description, :prompt, :render_status, :video_url, :parent_project_id)
        RETURNING id, created_at, updated_at`

	// NamedQuery works well with struct tags if fields match column names.
	// db.ManimProject already has sql.NullString for ParentProjectID, which sqlx handles correctly.
	rows, err := db.DB.NamedQuery(query, project)
	if err != nil {
		log.Errorf("Error creating Manim project: %v", err)
		return nil, fmt.Errorf("failed to create project: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		// Scan returned id, created_at, updated_at into the project struct
		err := rows.StructScan(project)
		if err != nil {
			log.Errorf("Error scanning Manim project data after creation: %v", err)
			return nil, fmt.Errorf("error scanning project after creation: %w", err)
		}
	} else {
		log.Error("No rows returned after Manim project creation.")
		return nil, fmt.Errorf("no rows returned after project creation")
	}

	log.Infof("Manim project '%s' created for user ID: %s (ID: %s)", project.Name, project.UserID.String(), project.ID.String())
	return project, nil
}

// FindManimProjectByID retrieves a Manim project by its ID.
// Includes new 'parent_project_id' field in the SELECT.
func FindManimProjectByID(projectID uuid.UUID) (*db.ManimProject, error) {
	project := &db.ManimProject{}
	// Added parent_project_id to the SELECT statement
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at, parent_project_id FROM manim_projects WHERE id = $1`
	err := db.DB.Get(project, query, projectID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("Manim project with ID '%s' not found.", projectID.String())
			return nil, nil // Project not found
		}
		log.Errorf("Error finding Manim project by ID '%s': %v", projectID.String(), err)
		return nil, fmt.Errorf("error finding project by ID: %w", err)
	}
	return project, nil
}

// FindManimProjectsByUserID retrieves all Manim projects for a specific user ID.
// Includes new 'parent_project_id' field in the SELECT.
func FindManimProjectsByUserID(userID uuid.UUID) ([]db.ManimProject, error) {
	var projects []db.ManimProject
	// Added parent_project_id to the SELECT statement
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at, parent_project_id FROM manim_projects WHERE user_id = $1 ORDER BY created_at DESC`
	err := db.DB.Select(&projects, query, userID)
	if err != nil {
		log.Errorf("Error finding Manim projects for user ID '%s': %v", userID.String(), err)
		return nil, fmt.Errorf("error finding projects by user ID: %w", err)
	}
	return projects, nil
}

// FindManimProjectByNameAndUserID retrieves a Manim project by its name and user ID.
// Includes new 'parent_project_id' field in the SELECT.
func FindManimProjectByNameAndUserID(name string, userID uuid.UUID) (*db.ManimProject, error) {
	project := &db.ManimProject{}
	// Added parent_project_id to the SELECT statement
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at, parent_project_id FROM manim_projects WHERE name = $1 AND user_id = $2`
	err := db.DB.Get(project, query, name, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("Manim project with name '%s' not found for user ID '%s'.", name, userID.String())
			return nil, nil // Project not found
		}
		log.Errorf("Error finding Manim project by name '%s' for user ID '%s': %v", name, userID.String(), err)
		return nil, fmt.Errorf("error finding project by name and user ID: %w", err)
	}
	return project, nil
}

// FindManimProjectsByParentID retrieves all sub-projects for a given parent project ID.
// This is a new function to support decomposed complex animations.
func FindManimProjectsByParentID(parentProjectID uuid.UUID) ([]db.ManimProject, error) {
	var projects []db.ManimProject
	// Select all fields including parent_project_id, filtered by the parent_project_id column.
	query := `SELECT id, user_id, name, description, prompt, render_status, video_url, created_at, updated_at, parent_project_id FROM manim_projects WHERE parent_project_id = $1 ORDER BY created_at ASC`
	err := db.DB.Select(&projects, query, parentProjectID)
	if err != nil {
		log.Errorf("Error finding sub-projects for parent ID '%s': %v", parentProjectID.String(), err)
		return nil, fmt.Errorf("error finding sub-projects by parent ID: %w", err)
	}
	return projects, nil
}

// UpdateManimProject updates an existing Manim project in the database.
// Includes new 'parent_project_id' field in the UPDATE, allowing it to be changed (though rare for existing projects).
func UpdateManimProject(project *db.ManimProject) error {
	project.UpdatedAt = time.Now().UTC() // Ensure updated_at is refreshed

	query := `
        UPDATE manim_projects
        SET name = :name, description = :description, prompt = :prompt, render_status = :render_status,
            video_url = :video_url, updated_at = :updated_at, parent_project_id = :parent_project_id
        WHERE id = :id AND user_id = :user_id` // Keep user_id in WHERE for security/ownership

	result, err := db.DB.NamedExec(query, project)
	if err != nil {
		log.Errorf("Error updating Manim project with ID '%s': %v", project.ID.String(), err)
		return fmt.Errorf("failed to update project: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		log.Warnf("No Manim project found with ID '%s' for user ID '%s' for update (or no changes were made).", project.ID.String(), project.UserID.String())
		return sql.ErrNoRows // Indicate that no row was updated, likely due to ID/ownership mismatch
	}

	log.Infof("Manim project with ID '%s' updated.", project.ID.String())
	return nil
}

// DeleteManimProject (no changes needed here as it deletes by ID and user_id, unaffected by parent_project_id)
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