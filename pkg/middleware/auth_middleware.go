package middleware

import (
	"net/http"
	"strings"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/services" // For JWT service
	"github.com/ASHISH26940/manim-orchestrator-api/pkg/utils"     // For HTTP responses
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
)

// Gin context key for storing user claims.
const UserClaimsContextKey = "userClaims"

// AuthMiddleware is a Gin middleware to authenticate requests using JWT.
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			log.Debug("AuthMiddleware: Missing Authorization header.")
			utils.ResponseWithError(c, http.StatusUnauthorized, "Authorization header required", nil)
			c.Abort() // Stop processing this request
			return
		}

		// Expected format: "Bearer <token>"
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			log.Debugf("AuthMiddleware: Invalid Authorization header format: %s", authHeader)
			utils.ResponseWithError(c, http.StatusUnauthorized, "Invalid Authorization header format", nil)
			c.Abort()
			return
		}

		tokenString := parts[1]

		claims, err := services.ValidateToken(tokenString)
		if err != nil {
			log.Debugf("AuthMiddleware: Invalid or expired JWT token: %v", err)
			utils.ResponseWithError(c, http.StatusUnauthorized, "Invalid or expired token", err.Error())
			c.Abort()
			return
		}

		// Store claims in context for downstream handlers
		c.Set(UserClaimsContextKey, claims)

		log.Debugf("AuthMiddleware: User %s (ID: %s) authenticated successfully.", claims.Email, claims.UserID.String())

		c.Next() // Continue to the next handler
	}
}

// GetUserClaimsFromContext extracts user claims from Gin context.
func GetUserClaimsFromContext(c *gin.Context) (*services.Claims, bool) {
	claims, exists := c.Get(UserClaimsContextKey)
	if !exists {
		return nil, false
	}
	userClaims, ok := claims.(*services.Claims)
	if !ok {
		return nil, false
	}
	return userClaims, true
}