package handlers

import (
	"net/http"
	"os"
	"strings"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db" // For CreateUser function
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/db/queries"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/services"
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils" // For common HTTP responses
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt" // For password hashing
)

var jwtSecret = []byte(os.Getenv("JWT_SECRET")) // Replace with your actual secret!


type UserClaims struct {
    Email    string `json:"email"`
    Username string `json:"username"`
    // Standard JWT claims (optional but good practice for 'exp', 'sub', 'iat', etc.)
    jwt.RegisteredClaims
}

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

func DeleteUser(c *gin.Context) {
    // --- 1. Extract User Claims from Gin Context (provided by AuthMiddleware) ---
    claimsAny, exists := c.Get("userClaims")
    if !exists {
        log.Error("DeleteUser: User claims not found in context. AuthMiddleware likely failed or wasn't applied correctly.")
        utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: User session data missing.", nil)
        return
    }

    // Ensure you import "github.com/ASHISH26940/manim-orchestrator-api/pkg/types"
    // and that your AuthMiddleware is setting *types.Claims
    verifiedClaims, ok := claimsAny.(*services.Claims)
    if !ok {
        log.Errorf("DeleteUser: Could not assert user claims from context to *types.Claims. Actual Type: %T", claimsAny)
        utils.ResponseWithError(c, http.StatusInternalServerError, "Authentication error: Invalid user session data format.", nil)
        return
    }

    verifiedUserEmail := verifiedClaims.Email
    verifiedUserID := verifiedClaims.Subject

    log.Infof("DeleteUser: Attempting deletion for user email: '%s', ID: '%s' (from context)", verifiedUserEmail, verifiedUserID)

    // Find the user by the VERIFIED email (from the context/token)
    userToDelete, err := queries.FindUserByEmail(verifiedUserEmail)
    if err != nil {
        log.Errorf("DeleteUser: Error finding user from verified email '%s': %v", verifiedUserEmail, err)
        utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to find user account", nil)
        return
    }
    if userToDelete == nil {
        log.Errorf("DeleteUser: User from verified token email '%s' not found in DB. Data inconsistency? User might have been deleted already.", verifiedUserEmail)
        utils.ResponseWithSuccess(c, http.StatusNotFound, "User account not found or already deleted.", nil)
        return
    }

    // --- REMOVE THE PASSWORD CONFIRMATION STEP ---
    // NO req.Email = strings.ToLower(req.Email)
    // NO if verifiedUserEmail != req.Email { ... } (This check is still good if you want to ensure the token holder deletes their *own* account without a body)
    // NO bcrypt.CompareHashAndPassword here.
    // If you remove the body, there's no `req.Email` to compare against anyway.
    // The *only* source of identity for the user now is the JWT token itself.

    // --- Proceed with Deletion ---
    err = queries.DeleteUser(userToDelete.ID)
    if err != nil {
        log.Errorf("DeleteUser: Error deleting user with ID '%s' (email: %s): %v", userToDelete.ID.String(), verifiedUserEmail, err)
        utils.ResponseWithError(c, http.StatusInternalServerError, "Failed to delete user account", nil)
        return
    }

    log.Infof("DeleteUser: User with ID '%s' (email: '%s') deleted successfully.", userToDelete.ID.String(), verifiedUserEmail)
    utils.ResponseWithSuccess(c, http.StatusNoContent, "User account deleted successfully", nil)
}