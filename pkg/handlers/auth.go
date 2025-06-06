package handlers

import (
	"net/http"
	"strings"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db" // For CreateUser function
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db/queries"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/services"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils" // For common HTTP responses
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt" // For password hashing
)

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=100"`
}
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func LoginUser(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Debugf("LoginUser: Invalid request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	req.Email = strings.ToLower(req.Email)

	// Find the user by email
	user, err := queries.FindUserByEmail(req.Email)
	if err != nil {
		log.Errorf("LoginUser: Error finding user by email: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Login failed", nil) // Generic error for security
		return
	}
	if user == nil {
		log.Debugf("LoginUser: User with email '%s' not found.", req.Email)
		utils.ResponseWithError(c, http.StatusUnauthorized, "Invalid credentials", nil)
		return
	}

	// Compare the provided password with the stored hash
	if err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		log.Debugf("LoginUser: Invalid password for user '%s'.", req.Email)
		utils.ResponseWithError(c, http.StatusUnauthorized, "Invalid credentials", nil)
		return
	}

	// Generate a JWT token
	token, err := services.GenerateToken(user.ID, user.Email, user.Username)
	if err != nil {
		log.Errorf("LoginUser: Failed to generate JWT token for user %s: %v", user.Email, err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to generate authentication token", nil)
		return
	}

	log.Infof("User %s logged in successfully.", user.Email)
	utils.ResponseWithSuccess(c, http.StatusOK, "Login successful", gin.H{"token": token})
}

func RegisterUser(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Debugf("Invalid request body: %v", err)
		utils.ResponseWithError(c, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}
	req.Email = strings.ToLower(req.Email)
	existingUser, err := queries.FindUserByEmail(req.Email)
	if err != nil {
		log.Errorf("Error finding user by email '%s': %v", req.Email, err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Error finding user by email", err.Error())
		return
	}
	if existingUser != nil {
		log.Debugf("User with email '%s' already exists.", req.Email)
		utils.ResponseWithError(c, http.StatusConflict, "User with email already exists", nil)
		return
	}
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Errorf("Error hashing password: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Error hashing password", err.Error())
		return
	}

	user := &db.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
	}

	createdUser, err := queries.CreateUser(user)
	if err != nil {
		log.Errorf("Error creating user: %v", err)
		utils.ResponseWithError(c, http.StatusInternalServerError, "Error creating user", err.Error())
		return
	}
	log.Infof("User with ID '%s' created.", createdUser.ID.String())

	utils.ResponseWithSuccess(c, http.StatusCreated, "User created successfully", nil)
}
