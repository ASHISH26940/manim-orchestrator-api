package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/config"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db/queries"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/llm"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/middleware"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)


type Handlers struct {
	Config    *config.Config
	LLMClient *llm.Service
}
// --- Request/Response Structs ---// Handlers struct to hold dependencies


// NewHandlers creates a new instance of Handlers
func NewHandlers(cfg *config.Config, llmClient *llm.Service) *Handlers {
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


// Request payload structure for merging videos
type MergeVideoRequest struct {
	IDs []string `json:"ids"` // List of video IDs (likely UUID strings) to merge
}

// Response payload structure from the Python renderer
type PythonMergeResponse struct {
	Message        string `json:"message"`
	MergedVideoID  string `json:"merged_video_id"`  // The UUID of the merged video
	MergedVideoURL string `json:"merged_video_url"` // The R2 URL from Python
	Error          string `json:"error"`             // Python might send an 'error' field
}

// Final response structure for frontend
type MergedVideoResponse struct {
	Message        string `json:"message"`
	MergedVideoID  string `json:"merged_video_id"`
	MergedVideoURL string `json:"merged_video_url"` // This will be the transformed R2 URL sent to frontend
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
		pr := newProjectResponse(&p) // Create the initial response object

		// --- URL TRANSFORMATION LOGIC ---
		// Check if VideoURL exists and contains the old domain
		if pr.VideoURL != "" && strings.Contains(pr.VideoURL, "41eca3477bd94f0eb869bef997e35147.r2.dev") {
			pr.VideoURL = strings.Replace(
				pr.VideoURL,
				"https://41eca3477bd94f0eb869bef997e35147.r2.dev",
				"https://pub-b0b0ca8b1fc2487b82486c56d37c2667.r2.dev",
				1, // Only replace the first occurrence (the domain prefix)
			)
		}
		// --- END URL TRANSFORMATION LOGIC ---

		projectResponses[i] = pr
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

// --- MergeVideosHandler (Auth Check Removed) ---
func (h *Handlers) MergeVideosHandler(c *gin.Context) {
	// --- AUTHENTICATION AND USER CLAIMS CHECK REMOVED ---
	// You might also remove middleware.GetUserClaimsFromContext if it's not used anywhere else
	// and you remove related import.

	// 1. Parse the incoming request body from the frontend
	var req MergeVideoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Errorf("MergeVideosHandler: Invalid request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid request body. 'ids' (list of video IDs) is required.", err.Error())
		return
	}

	if len(req.IDs) == 0 {
		log.Warn("MergeVideosHandler: No video IDs provided for merging.")
		utils.ResponseWithError(c, http.StatusBadRequest, "No video IDs provided for merging.", nil)
		return
	}

	// --- OPTIONAL: OWNERSHIP VALIDATION REMOVED ---
	// Since there's no user authenticated, you cannot validate ownership against a user ID.
	// If you still need to ensure videos exist, you'd perform queries.FindManimProjectByID
	// for each ID without checking `project.UserID` against `claims.UserID`.
	/*
		for _, videoIDStr := range req.IDs {
			videoID, err := uuid.Parse(videoIDStr)
			if err != nil {
				log.Warnf("MergeVideosHandler: Invalid video ID format '%s': %v", videoIDStr, err)
				utils.ResponseWithError(c, http.StatusBadRequest, fmt.Sprintf("Invalid video ID format: %s", videoIDStr), nil)
				return
			}
			// This check for `project.UserID != claims.UserID` is no longer applicable
			// without `claims`. If you still want to ensure projects exist,
			// just remove the `claims.UserID` part.
			project, err := queries.FindManimProjectByID(videoID)
			if err != nil {
				log.Errorf("MergeVideosHandler: Failed to fetch video/project %s for existence check: %v", videoID.String(), err)
				utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to verify video existence", nil)
				return
			}
			if project == nil {
				log.Warnf("MergeVideosHandler: Video/project %s not found.", videoID.String())
				utils.ResponseWithError(c, http.StatusNotFound, fmt.Sprintf("Video ID not found: %s", videoID.String()), nil)
				return
			}
		}
		log.Infof("MergeVideosHandler: Verified existence for %d video IDs.", len(req.IDs))
	*/


	// 2. Get the Python renderer URL for merging from your config
	pythonMergeRendererURL := h.Config.ManimRendererURL
	if pythonMergeRendererURL == "" {
		log.Error("MergeVideosHandler: h.Config.ManimRendererURL is not set. Cannot proceed with merging.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Backend configuration error: Python renderer URL for merging not set.", nil)
		return
	}
	log.Infof("MergeVideosHandler: Using Python renderer URL for merging from config: %s", pythonMergeRendererURL)

	// Fetch R2 domain configuration from environment variables (consider moving to h.Config)
	pythonR2InternalDomain := os.Getenv("PYTHON_R2_INTERNAL_DOMAIN")
	frontendR2PublicDomain := os.Getenv("FRONTEND_R2_PUBLIC_DOMAIN")

	if pythonR2InternalDomain == "" || frontendR2PublicDomain == "" {
		log.Warn("MergeVideosHandler: PYTHON_R2_INTERNAL_DOMAIN or FRONTEND_R2_PUBLIC_DOMAIN not set. Merged video URL will not be transformed for frontend display.")
	}

	// 3. Prepare the request payload to send to the Python renderer
	payloadBytes, err := json.Marshal(req)
	if err != nil {
		log.Errorf("MergeVideosHandler: Failed to marshal payload for Python renderer: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Internal server error preparing merge request.", nil)
		return
	}

	// Construct the full endpoint for the merge operation on the Python renderer
	flaskEndpoint := fmt.Sprintf("%s/merge_videos", pythonMergeRendererURL)
	log.Infof("MergeVideosHandler: Forwarding merge request to Python renderer at: %s with IDs: %v", flaskEndpoint, req.IDs)

	// 4. Make the HTTP POST request to the Python renderer
	client := &http.Client{Timeout: 60 * time.Second} // Give Python some time to merge
	resp, err := client.Post(flaskEndpoint, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		log.Errorf("MergeVideosHandler: Failed to connect to Python renderer at %s: %v", flaskEndpoint, err)
		utils.ResponseWithError(c, http.StatusBadGateway, "Failed to connect to video processing service for merging.", nil)
		return
	}
	defer resp.Body.Close()

	// 5. Read and parse the response from the Python renderer
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("MergeVideosHandler: Failed to read response from Python renderer: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Error reading response from video merging service.", nil)
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Errorf("MergeVideosHandler: Python renderer returned status %d with body: %s", resp.StatusCode, string(responseBody))
		var pythonErrorResp PythonMergeResponse
		if jsonErr := json.Unmarshal(responseBody, &pythonErrorResp); jsonErr == nil && pythonErrorResp.Error != "" {
			utils.ResponseWithError(c, resp.StatusCode, pythonErrorResp.Error, nil)
		} else {
			utils.ResponseWithError(c, resp.StatusCode, "Video merging service reported an error.", string(responseBody))
		}
		return
	}

	// 6. Successfully merged - parse Python's success response
	var pythonSuccessResp PythonMergeResponse
	if err := json.Unmarshal(responseBody, &pythonSuccessResp); err != nil {
		log.Errorf("MergeVideosHandler: Failed to unmarshal success response from Python renderer: %v. Body: %s", err, string(responseBody))
		utils.ResponseWithError(c, http.StatusInternalServerError, "Error parsing successful merge response from Python.", nil)
		return
	}

	// --- PERFORM THE URL TRANSFORMATION HERE ---
	finalURLForFrontend := pythonSuccessResp.MergedVideoURL
	if pythonSuccessResp.MergedVideoURL != "" && pythonR2InternalDomain != "" && frontendR2PublicDomain != "" {
		parsedURL, err := url.Parse(pythonSuccessResp.MergedVideoURL)
		if err != nil {
			log.Warnf("MergeVideosHandler: Could not parse merged video URL from Python: %s. Error: %v. Skipping transformation.", pythonSuccessResp.MergedVideoURL, err)
		} else {
			internalDomain := strings.TrimSuffix(pythonR2InternalDomain, "/")
			publicDomain := strings.TrimSuffix(frontendR2PublicDomain, "/")

			if strings.EqualFold(parsedURL.Scheme+"://"+parsedURL.Host, internalDomain) {
				originalURL := pythonSuccessResp.MergedVideoURL
				finalURLForFrontend = fmt.Sprintf("%s%s", publicDomain, parsedURL.Path)
				log.Infof("MergeVideosHandler: Transformed URL from %s to %s", originalURL, finalURLForFrontend)
			} else {
				log.Warnf("MergeVideosHandler: Merged video URL '%s' does not use expected internal domain '%s'. No transformation applied.", pythonSuccessResp.MergedVideoURL, internalDomain)
			}
		}
	} else if pythonSuccessResp.MergedVideoURL != "" {
		log.Warn("MergeVideosHandler: Domain transformation skipped due to missing environment variables. Merged video URL is not transformed.")
	}
	// --- END URL TRANSFORMATION ---

	// --- Store the final R2 URL in Neon PostgreSQL using your 'db' package ---
	if db.DB == nil {
		log.Error("MergeVideosHandler: Database connection (db.DB) is not initialized.")
		utils.ResponseWithError(c, http.StatusInternalServerError, "Database connection error.", nil)
		return
	}

	query := `INSERT INTO merged_videos (id, r2_url) VALUES (:id, :r2_url) ON CONFLICT (id) DO UPDATE SET r2_url = EXCLUDED.r2_url;`

	_, err = db.DB.NamedExec(query, map[string]interface{}{
		"id":     pythonSuccessResp.MergedVideoID,
		"r2_url": finalURLForFrontend,
	})
	if err != nil {
		log.Errorf("MergeVideosHandler: Failed to insert/update merged video URL in Neon DB: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to record merged video in database.", nil)
		return
	}
	log.Infof("MergeVideosHandler: Successfully stored R2 URL '%s' for ID '%s' in Neon DB.", finalURLForFrontend, pythonSuccessResp.MergedVideoID)
	// --- END Neon PostgreSQL Storage ---

	// 7. Respond to the frontend with the merged video details
	log.Infof("MergeVideosHandler: Successfully merged videos. Final URL for frontend: %s", finalURLForFrontend)
	finalResponse := MergedVideoResponse{
		Message:        "Videos merged, uploaded to R2, and URL recorded in Neon successfully.",
		MergedVideoID:  pythonSuccessResp.MergedVideoID,
		MergedVideoURL: finalURLForFrontend, // This is the transformed R2 URL
	}
	utils.ResponseWithSuccess(c, http.StatusOK, "Videos merged and uploaded successfully", finalResponse)
}