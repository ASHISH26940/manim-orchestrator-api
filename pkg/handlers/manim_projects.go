package handlers

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/llm"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/config"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db/queries"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/middleware"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)


type Handlers struct {
	Config    *config.Config
	LLMClient *llm.LLMClient
}
// --- Request/Response Structs ---// Handlers struct to hold dependencies


// NewHandlers creates a new instance of Handlers
func NewHandlers(cfg *config.Config, llmClient *llm.LLMClient) *Handlers {
	return &Handlers{
		Config:    cfg,
		LLMClient: llmClient,
	}
}



// CreateProjectRequest defines the structure for creating a new Manim project.
type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required,min=3,max=255"`
	Description string `json:"description"`
	Prompt      string `json:"prompt" binding:"required,min=10"` // Prompt for Manim code generation
}

// UpdateProjectRequest defines the structure for updating an existing Manim project.
type UpdateProjectRequest struct {
	Name        *string `json:"name" binding:"omitempty,min=3,max=255"` // Pointers to allow partial updates
	Description *string `json:"description"`
	Prompt      *string `json:"prompt" binding:"omitempty,min=10"`
	// RenderStatus and VideoURL will be updated internally by the orchestrator, not directly by user via this endpoint
}

// ProjectResponse defines the structure for sending Manim project data back to the client.
type ProjectResponse struct {
	ID           uuid.UUID `json:"id"`
	UserID       uuid.UUID `json:"user_id"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Prompt       string    `json:"prompt"`
	RenderStatus string    `json:"render_status"`
	VideoURL     string    `json:"video_url"`
	CreatedAt    string    `json:"created_at"` // Using string for formatted timestamp
	UpdatedAt    string    `json:"updated_at"`
}

// newProjectResponse converts a db.ManimProject to a ProjectResponse.
func newProjectResponse(project *db.ManimProject) ProjectResponse {
	videoURL:=""
	if project.VideoURL.Valid{
		videoURL=project.VideoURL.String
	}
	return ProjectResponse{
		ID:           project.ID,
		UserID:       project.UserID,
		Name:         project.Name,
		Description:  project.Description,
		Prompt:       project.Prompt,
		RenderStatus: project.RenderStatus,
		VideoURL:     videoURL,
		CreatedAt:    project.CreatedAt.Format(http.TimeFormat), // Standard HTTP time format
		UpdatedAt:    project.UpdatedAt.Format(http.TimeFormat),
	}
}

// --- API Handlers ---

// CreateManimProject handles the creation of a new Manim project.
func CreateManimProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf("CreateManimProject: Invalid request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("CreateManimProject: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	// Check if a project with the same name already exists for this user
	existingProject, err := queries.FindManimProjectByNameAndUserID(req.Name, claims.UserID)
	if err != nil && err != sql.ErrNoRows {
		log.Errorf("CreateManimProject: Database error checking existing project: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to check project existence", nil)
		return
	}
	if existingProject != nil {
		log.Debugf("CreateManimProject: Project with name '%s' already exists for user %s.", req.Name, claims.UserID.String())
		utils.ResponseWithError(c, http.StatusConflict, "Project with this name already exists for your account", nil)
		return
	}

	project := &db.ManimProject{
		UserID:      claims.UserID,
		Name:        strings.TrimSpace(req.Name), // Trim whitespace
		Description: strings.TrimSpace(req.Description),
		Prompt:      strings.TrimSpace(req.Prompt),
		RenderStatus: "pending", // Default status for new projects
		VideoURL:    sql.NullString{Valid: false},        // No video URL initially
	}

	createdProject, err := queries.CreateManimProject(project)
	if err != nil {
		log.Errorf("CreateManimProject: Failed to create project in DB: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to create Manim project", nil)
		return
	}

	log.Infof("Manim project '%s' created successfully for user %s. ID: %s", createdProject.Name, claims.UserID.String(), createdProject.ID.String())
	utils.ResponseWithSuccess(c, http.StatusCreated, "Manim project created successfully", newProjectResponse(createdProject))
}

// GetUserManimProjects handles fetching all Manim projects for the authenticated user.
func GetUserManimProjects(c *gin.Context) {
	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("GetUserManimProjects: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	projects, err := queries.FindManimProjectsByUserID(claims.UserID)
	if err != nil {
		log.Errorf("GetUserManimProjects: Failed to fetch projects for user %s: %v", claims.UserID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to retrieve Manim projects", nil)
		return
	}

	// Convert db.ManimProject slice to ProjectResponse slice
	projectResponses := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		projectResponses[i] = newProjectResponse(&p)
	}

	log.Infof("Found %d projects for user %s.", len(projects), claims.UserID.String())
	utils.ResponseWithSuccess(c, http.StatusOK, "Manim projects retrieved successfully", projectResponses)
}

// GetManimProjectByID handles fetching a single Manim project by its ID, ensuring ownership.
func GetManimProjectByID(c *gin.Context) {
	projectIDParam := c.Param("id") // Get ID from URL path
	projectID, err := uuid.Parse(projectIDParam)
	if err != nil {
		log.Warnf("GetManimProjectByID: Invalid project ID format '%s': %v", projectIDParam, err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid project ID format", nil)
		return
	}

	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("GetManimProjectByID: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	project, err := queries.FindManimProjectByID(projectID)
	if err != nil {
		log.Errorf("GetManimProjectByID: Failed to fetch project %s: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to retrieve Manim project", nil)
		return
	}
	if project == nil {
		log.Debugf("GetManimProjectByID: Project with ID %s not found.", projectID.String())
		utils.ResponseWithError(c, http.StatusNotFound, "Manim project not found", nil)
		return
	}

	// IMPORTANT: Ensure the retrieved project belongs to the authenticated user
	if project.UserID != claims.UserID {
		log.Warnf("GetManimProjectByID: User %s attempted to access project %s owned by %s.", claims.UserID.String(), projectID.String(), project.UserID.String())
		utils.ResponseWithError(c, http.StatusForbidden, "You do not have permission to access this project", nil)
		return
	}

	log.Infof("Retrieved project %s for user %s.", projectID.String(), claims.UserID.String())
	utils.ResponseWithSuccess(c, http.StatusOK, "Manim project retrieved successfully", newProjectResponse(project))
}

// UpdateManimProject handles updating an existing Manim project, ensuring ownership.
func UpdateManimProject(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := uuid.Parse(projectIDParam)
	if err != nil {
		log.Warnf("UpdateManimProject: Invalid project ID format '%s': %v", projectIDParam, err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid project ID format", nil)
		return
	}

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warnf("UpdateManimProject: Invalid request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("UpdateManimProject: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	// Fetch the existing project to get current values and ensure ownership
	existingProject, err := queries.FindManimProjectByID(projectID)
	if err != nil {
		log.Errorf("UpdateManimProject: Database error fetching project %s: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to check project existence", nil)
		return
	}
	if existingProject == nil {
		log.Debugf("UpdateManimProject: Project with ID %s not found.", projectID.String())
		utils.ResponseWithError(c, http.StatusNotFound, "Manim project not found", nil)
		return
	}

	// IMPORTANT: Ensure the project belongs to the authenticated user
	if existingProject.UserID != claims.UserID {
		log.Warnf("UpdateManimProject: User %s attempted to update project %s owned by %s.", claims.UserID.String(), projectID.String(), existingProject.UserID.String())
		utils.ResponseWithError(c, http.StatusForbidden, "You do not have permission to modify this project", nil)
		return
	}

	// Apply updates only if fields are provided in the request
	if req.Name != nil {
		// Check for name conflict if name is being updated
		if strings.TrimSpace(*req.Name) != existingProject.Name { // Only check if name is actually changing
			conflictProject, err := queries.FindManimProjectByNameAndUserID(strings.TrimSpace(*req.Name), claims.UserID)
			if err != nil && err != sql.ErrNoRows {
				log.Errorf("UpdateManimProject: Database error checking name conflict: %v", err)
				utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to check name conflict", nil)
				return
			}
			if conflictProject != nil && conflictProject.ID != existingProject.ID { // Ensure it's not the same project
				log.Debugf("UpdateManimProject: New name '%s' already exists for another project of user %s.", *req.Name, claims.UserID.String())
				utils.ResponseWithError(c, http.StatusConflict, "Another project with this name already exists for your account", nil)
				return
			}
		}
		existingProject.Name = strings.TrimSpace(*req.Name)
	}
	if req.Description != nil {
		existingProject.Description = strings.TrimSpace(*req.Description)
	}
	if req.Prompt != nil {
		existingProject.Prompt = strings.TrimSpace(*req.Prompt)
	}

	err = queries.UpdateManimProject(existingProject)
	if err != nil {
		if err == sql.ErrNoRows { // This would imply a race condition where it was deleted after fetching, unlikely if ownership is checked
			log.Warnf("UpdateManimProject: Project %s disappeared during update process.", projectID.String())
			utils.ResponseWithError(c, http.StatusNotFound, "Manim project not found for update", nil)
			return
		}
		log.Errorf("UpdateManimProject: Failed to update project %s in DB: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to update Manim project", nil)
		return
	}

	log.Infof("Manim project %s updated successfully for user %s.", projectID.String(), claims.UserID.String())
	utils.ResponseWithSuccess(c, http.StatusOK, "Manim project updated successfully", newProjectResponse(existingProject))
}

// DeleteManimProject handles deleting an existing Manim project, ensuring ownership.
func DeleteManimProject(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := uuid.Parse(projectIDParam)
	if err != nil {
		log.Warnf("DeleteManimProject: Invalid project ID format '%s': %v", projectIDParam, err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid project ID format", nil)
		return
	}

	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("DeleteManimProject: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	// No need to fetch the project first, as the queries.DeleteManimProject function
	// already includes the user_id in its WHERE clause to enforce ownership.
	err = queries.DeleteManimProject(projectID, claims.UserID)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Debugf("DeleteManimProject: Project with ID %s not found or not owned by user %s.", projectID.String(), claims.UserID.String())
			utils.ResponseWithError(c, http.StatusNotFound, "Manim project not found or you do not have permission to delete it", nil)
			return
		}
		log.Errorf("DeleteManimProject: Failed to delete project %s for user %s: %v", projectID.String(), claims.UserID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to delete Manim project", nil)
		return
	}

	log.Infof("Manim project %s deleted successfully for user %s.", projectID.String(), claims.UserID.String())
	utils.ResponseWithSuccess(c, http.StatusNoContent, "Manim project deleted successfully", nil) // 204 No Content for successful deletion
}

// RendererResponse defines the expected structure of the response from the Python Manim Renderer service.
type RendererResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	LocalVideoPath string `json:"local_video_path"` // This will be the R2 URL later
}


func (h *Handlers) TriggerManimGenerationAndRender(c *gin.Context) {
	projectIDParam := c.Param("id")
	projectID, err := uuid.Parse(projectIDParam)
	if err != nil {
		log.Warnf("TriggerManimGenerationAndRender: Invalid project ID format '%s': %v", projectIDParam, err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid project ID format", nil)
		return
	}

	claims, exists := middleware.GetUserClaimsFromContext(c)
	if !exists {
		log.Error("TriggerManimGenerationAndRender: User claims not found in context.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User claims not found", nil)
		return
	}

	// 1. Fetch the project and check ownership
	project, err := queries.FindManimProjectByID(projectID)
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to fetch project %s: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to retrieve Manim project", nil)
		return
	}
	if project == nil {
		log.Debugf("TriggerManimGenerationAndRender: Project with ID %s not found.", projectID.String())
		utils.ResponseWithError(c, http.StatusNotFound, "Manim project not found", nil)
		return
	}
	if project.UserID != claims.UserID {
		log.Warnf("TriggerManimGenerationAndRender: User %s attempted to trigger render for project %s owned by %s.", claims.UserID.String(), projectID.String(), project.UserID.String())
		utils.ResponseWithError(c, http.StatusForbidden, "You do not have permission to trigger generation for this project", nil)
		return
	}

	// Check if prompt is empty
	if strings.TrimSpace(project.Prompt) == "" {
		log.Warnf("TriggerManimGenerationAndRender: Project %s has an empty prompt.", projectID.String())
		utils.ResponseWithError(c, http.StatusBadRequest, "Project prompt is empty. Please update the project with a valid prompt.", nil)
		return
	}

	// 2. Update project status to indicate generation is in progress (optional for this test, but good practice)
	// For this test, we might not persist this status to avoid cluttering DB with 'generating' without rendering.
	// But keeping it as a conceptual step.
	log.Infof("Attempting to generate Manim code for project %s with prompt: %s", projectID.String(), project.Prompt)

	// 3. Generate Manim code using LLM
	generatedManimCode, err := h.LLMClient.GenerateManimCode(project.Prompt)
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to generate Manim code for project %s: %v", projectID.String(), err)
		// No need to update DB status to failed here, as we are only testing generation.
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to generate Manim code", nil)
		return
	}
	log.Infof("Manim code generated for project %s. Length: %d", projectID.String(), len(generatedManimCode))

	// 4. Send the generated code as a response for verification
	// THIS IS THE TEMPORARY PART - instead of calling renderer, we return the code.
	utils.ResponseWithSuccess(c, http.StatusOK, "Manim code generated successfully (rendering skipped for test)", gin.H{
		"project_id": projectID.String(),
		"prompt":     project.Prompt,
		"generated_manim_code": generatedManimCode,
		"status":     "code_generated_only_for_test",
	})

	// ALL THE FOLLOWING STEPS (calling renderer, updating status to rendering/completed, etc.)
	// ARE SKIPPED IN THIS TEMPORARY VERSION.
	// You will uncomment and re-enable them after verifying code generation.
}