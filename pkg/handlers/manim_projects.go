package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"fmt"
	"bytes"
	"encoding/json"
	"time"
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

type RendererRequest struct {
	ProjectID     string `json:"project_id"`
	ScriptContent string `json:"script_content"`
	CallbackURL   string `json:"callback_url"`
}

// RenderCallbackRequest defines the expected structure of the POST request from the Python renderer to our callback endpoint.
type RenderCallbackRequest struct {
	ProjectID    string `json:"project_id"`
	Status       string `json:"status"` // e.g., "completed", "failed", "upload_failed", etc.
	VideoURL     string `json:"video_url"` // Will be the R2 public URL on success, "N/A" or empty on failure
	Message      string `json:"message"` // General message from renderer
	ErrorDetails string `json:"error_details"` // Optional, for specific error info
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


// --- REVERTED/UPDATED: TriggerManimGenerationAndRender Handler ---
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
		utils.ResponseWithError(c, http.StatusForbidden, "You do not have permission to trigger rendering for this project", nil)
		return
	}

	// Check if prompt is empty
	if strings.TrimSpace(project.Prompt) == "" {
		log.Warnf("TriggerManimGenerationAndRender: Project %s has an empty prompt.", projectID.String())
		utils.ResponseWithError(c, http.StatusBadRequest, "Project prompt is empty. Please update the project with a valid prompt.", nil)
		return
	}

	// 2. Update project status to indicate generation is in progress
	project.RenderStatus = "generating"
	err = queries.UpdateManimProject(project) // Update the status in DB
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to update project %s status to 'generating': %v", projectID.String(), err)
		// Continue as this is a best effort update, but log it
	}
	log.Infof("Project %s status updated to 'generating'.", projectID.String())


	// --- Start of LLM Generation & Renderer Trigger ---

	// 3. Generate Manim code using LLM
	generatedManimCode, err := h.LLMClient.GenerateManimCode(project.Prompt)
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to generate Manim code for project %s: %v", projectID.String(), err)
		project.RenderStatus = "failed: code_gen_error"
		queries.UpdateManimProject(project) // Best effort update
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to generate Manim code", nil)
		return
	}
	log.Infof("Manim code generated for project %s. Length: %d", projectID.String(), len(generatedManimCode))

	callbackHost := h.Config.Host // Default assuming Go is in Docker-Compose where HOST is its service name
    if h.Config.Host == "127.0.0.1" || h.Config.Host == "0.0.0.0" { // ADDED "0.0.0.0" check
        // This is a common pattern for Docker Desktop to reach host services.
        // For production or pure Linux setups, review your networking.
        callbackHost = "host.docker.internal"
    }
    // IMPORTANT: Make sure the callback endpoint is correctly configured in your Go router
    // to match this URL structure (e.g., /api/projects/render-callback).
    callbackURL := fmt.Sprintf("http://%s:%s/api/projects/render-callback", callbackHost, h.Config.Port)


	rendererReqBody := RendererRequest{
		ProjectID:     project.ID.String(),
		ScriptContent: generatedManimCode,
		CallbackURL:   callbackURL,
	}
	log.Debugf("%s",rendererReqBody)

	jsonBody, _ := json.Marshal(rendererReqBody)
	
	client := &http.Client{Timeout: 10 * time.Second} // Shorter timeout for initial request, as rendering is async
	rendererURL := fmt.Sprintf("%s/render", h.Config.ManimRendererURL) // ManimRendererURL from config

	req, err := http.NewRequest("POST", rendererURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to create request to renderer: %v", err)
		project.RenderStatus = "failed: renderer_req_error"
		queries.UpdateManimProject(project)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to prepare render request", nil)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("TriggerManimGenerationAndRender: Failed to send request to renderer %s: %v", rendererURL, err)
		project.RenderStatus = "failed: renderer_comm_error"
		queries.UpdateManimProject(project)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to connect to Manim renderer", nil)
		return
	}
	defer resp.Body.Close()

	// The renderer will respond immediately with 202 Accepted
	if resp.StatusCode != http.StatusAccepted { // Expected 202
		var errorResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errorResp)
		errMsg := errorResp["error"]
		if errMsg == "" {
			errMsg = "Unknown error from renderer."
		}
		log.Errorf("TriggerManimGenerationAndRender: Renderer returned unexpected status %d: %s", resp.StatusCode, errMsg)
		project.RenderStatus = fmt.Sprintf("failed: renderer_status_%d", resp.StatusCode)
		queries.UpdateManimProject(project)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to start Manim rendering process", errMsg)
		return
	}

	// 5. Respond immediately to the client that rendering has started (asynchronous)
	log.Infof("Manim rendering process initiated for project %s. Renderer returned 202 Accepted.", projectID.String())
	utils.ResponseWithSuccess(c, http.StatusAccepted, "Manim rendering process initiated", gin.H{
		"project_id": projectID.String(),
		"status":     "rendering_initiated",
		"message":    "Manim rendering is in progress. The video URL will be updated via callback.",
	})
	// --- End of LLM Generation & Renderer Trigger ---
}


// --- NEW: HandleRenderCallback Handler ---
// This endpoint receives the result of the Manim rendering from the Python service.
func (h *Handlers) HandleRenderCallback(c *gin.Context) {
	var callback RenderCallbackRequest // Use the struct defined above
	if err := c.ShouldBindJSON(&callback); err != nil {
		log.Errorf("HandleRenderCallback: Invalid callback request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid callback request body", err.Error())
		return
	}

	projectID, err := uuid.Parse(callback.ProjectID)
	if err != nil {
		log.Errorf("HandleRenderCallback: Invalid ProjectID in callback '%s': %v", callback.ProjectID, err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid ProjectID in callback", nil)
		return
	}

	log.Infof("Received render callback for Project ID: %s, Status: %s, VideoURL: %s",
		callback.ProjectID, callback.Status, callback.VideoURL)

	project, err := queries.FindManimProjectByID(projectID)
	if err != nil {
		log.Errorf("HandleRenderCallback: Failed to find project %s for callback: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to find project for callback", nil)
		return
	}
	if project == nil {
		log.Warnf("HandleRenderCallback: Project %s not found for callback. Perhaps already deleted?", projectID.String())
		utils.ResponseWithError(c, http.StatusNotFound, "Project not found for callback", nil)
		return
	}

	// Update project status based on callback
	project.RenderStatus = callback.Status
	if callback.Status == "completed" {
		// Only set video_url if status is completed and URL is not "N/A"
		if callback.VideoURL != "" && callback.VideoURL != "N/A" {
			project.VideoURL = sql.NullString{String: callback.VideoURL, Valid: true}
			log.Infof("Project %s render completed. Video URL: %s", projectID.String(), callback.VideoURL)
		} else {
			project.VideoURL = sql.NullString{Valid: false} // Ensure it's NULL if completed but no URL
			log.Warnf("Project %s completed, but no valid video URL provided in callback.", projectID.String())
		}
	} else {
		// Clear URL on failure/non-completed status
		project.VideoURL = sql.NullString{Valid: false}
		log.Errorf("Project %s rendering failed with status: %s. Details: %s", projectID.String(), callback.Status, callback.ErrorDetails)
	}

	// Important: The `updated_at` field will be automatically updated by the DB trigger
	// when we call queries.UpdateManimProject.

	err = queries.UpdateManimProject(project)
	if err != nil {
		log.Errorf("HandleRenderCallback: Failed to update project %s status and URL after callback: %v", projectID.String(), err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to update project after rendering callback", nil)
		return
	}

	utils.ResponseWithSuccess(c, http.StatusOK, "Callback processed successfully", nil)
}