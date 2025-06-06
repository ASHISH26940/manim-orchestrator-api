package services

import (
	"time"

	"github.com/ASHISH26940/manim-orchestrator-api/pkg/config" // To get JWT_SECRET
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid" // For user ID (if using UUIDs in claims)
	log "github.com/sirupsen/logrus"
)

// Claims defines the JWT claims (payload).
// We embed jwt.RegisteredClaims for standard claims like ExpiresAt, IssuedAt.
type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Email    string    `json:"email"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for a given user.
func GenerateToken(userID uuid.UUID, email, username string) (string, error) {
	// Get JWT secret from configuration
	cfg := config.LoadConfig()
	jwtSecret := []byte(cfg.JwtSecret)

	// Set token expiration (e.g., 24 hours from now)
	expirationTime := time.Now().Add(24 * time.Hour)

	// Create the claims
	claims := &Claims{
		UserID:   userID,
		Email:    email,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "manim-orchestrator-api",
			Subject:   userID.String(), // Subject is typically the user ID
		},
	}

	// Create the token with the claims and signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with the secret key
	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		log.Errorf("Failed to sign JWT token for user %s: %v", email, err)
		return "", err
	}

	log.Debugf("Generated JWT for user %s, expires at %s", email, expirationTime.Format(time.RFC3339))
	return tokenString, nil
}

// ValidateToken validates a JWT token and returns the claims if valid.
// (This function will be used in the JWT authentication middleware later)
func ValidateToken(tokenString string) (*Claims, error) {
	cfg := config.LoadConfig()
	jwtSecret := []byte(cfg.JwtSecret)

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return jwtSecret, nil
	})

	if err != nil {
		log.Warnf("JWT validation failed: %v", err)
		return nil, err
	}

	if !token.Valid {
		log.Warn("Invalid JWT token.")
		return nil, jwt.ErrInvalidKey
	}

	return claims, nil
}