package auth

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// UserClaims represents the JWT claims for a user
type UserClaims struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates Supabase JWT tokens
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Extract token from "Bearer <token>"
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		// Parse and validate the token
		claims, err := validateJWT(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// Add user info to context
		c.Set("user_id", claims.Sub)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)

		c.Next()
	}
}

// OptionalAuthMiddleware validates JWT tokens but doesn't require them
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString != authHeader {
				claims, err := validateJWT(tokenString)
				if err == nil {
					c.Set("user_id", claims.Sub)
					c.Set("user_email", claims.Email)
					c.Set("user_role", claims.Role)
				}
			}
		}
		c.Next()
	}
}

// ValidateJWTToken validates a Supabase JWT token (exported for reuse)
func ValidateJWTToken(tokenString string) (*UserClaims, error) {
	return validateJWT(tokenString)
}

// validateJWT validates a Supabase JWT token
func validateJWT(tokenString string) (*UserClaims, error) {
	// Get the JWT secret from environment
	jwtSecret := os.Getenv("SUPABASE_JWT_SECRET")

	// Parse the token with HS256 signing method
	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*UserClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, jwt.ErrInvalidKey
}

// GetUserID extracts user ID from Gin context
func GetUserID(c *gin.Context) (string, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return "", false
	}

	id, ok := userID.(string)
	return id, ok
}

// GetUserEmail extracts user email from Gin context
func GetUserEmail(c *gin.Context) (string, bool) {
	userEmail, exists := c.Get("user_email")
	if !exists {
		return "", false
	}

	email, ok := userEmail.(string)
	return email, ok
}

// RequireAuth is a helper that checks if user is authenticated
func RequireAuth(c *gin.Context) (string, bool) {
	userID, exists := GetUserID(c)
	if !exists || userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return "", false
	}
	return userID, true
}
