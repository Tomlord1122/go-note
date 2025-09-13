package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"

	"go-note/internal/auth"
	"go-note/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthHandler handles OAuth-related HTTP requests
type OAuthHandler struct {
	userService *services.UserService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(db *pgxpool.Pool) (*OAuthHandler, error) {
	return &OAuthHandler{
		userService: services.NewUserService(db),
	}, nil
}

// GoogleLoginRequest represents the request body for Google login
type GoogleLoginRequest struct {
	RedirectURL string `json:"redirect_url,omitempty"`
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	URL         string `json:"url,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	User        *User  `json:"user,omitempty"`
	Message     string `json:"message,omitempty"`
}

// User represents user information from Supabase
type User struct {
	ID       string                 `json:"id"`
	Email    string                 `json:"email"`
	Metadata map[string]interface{} `json:"user_metadata,omitempty"`
}

// GoogleLogin handles POST /auth/google/login
// Initiates Google OAuth flow
func (h *OAuthHandler) GoogleLogin(c *gin.Context) {
	var req GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// If no body provided, use default redirect URL
		req.RedirectURL = os.Getenv("FRONTEND_URL")
		if req.RedirectURL == "" {
			req.RedirectURL = "http://localhost:5173"
		}
	}

	fmt.Println("RedirectURL", req.RedirectURL)
	// Generate OAuth URL manually since the Supabase client doesn't have direct OAuth URL generation
	supabaseURL := os.Getenv("SUPABASE_URL")
	redirectURL := req.RedirectURL + "/auth/callback"

	oauthURL := fmt.Sprintf("%s/auth/v1/authorize?provider=google&redirect_to=%s",
		supabaseURL,
		url.QueryEscape(redirectURL))

	c.JSON(http.StatusOK, AuthResponse{
		URL:     oauthURL,
		Message: "Redirect to Google OAuth",
	})
}

// GoogleCallback handles GET /auth/google/callback
// Handles the callback from Google OAuth
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	// Get tokens from URL fragments (they would be passed as query params in this case)
	accessToken := c.Query("access_token")
	_ = c.Query("refresh_token") // refresh token for future use

	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token is required"})
		return
	}

	// Parse the JWT token to extract user information
	claims, err := auth.ValidateJWTToken(accessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token format: " + err.Error()})
		return
	}

	userResponse := &User{
		ID:    claims.Sub,
		Email: claims.Email,
		Metadata: map[string]interface{}{
			"role": claims.Role,
		},
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken: accessToken,
		User:        userResponse,
		Message:     "Successfully authenticated",
	})
}

// RefreshTokenRequest represents the request body for token refresh
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshTokenResponse represents the response from token refresh
type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// RefreshToken handles POST /auth/refresh
// Self-managed JWT tokens (no ANON_KEY required!)
func (h *OAuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token is required"})
		return
	}

	// Initialize token manager
	tokenManager := auth.NewTokenManager()

	// Validate refresh token
	refreshClaims, err := tokenManager.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Invalid refresh token",
			"message": "Please re-authenticate using the login flow",
		})
		return
	}

	// Get user information from database to generate new tokens
	// You might want to fetch fresh user data here
	userID := refreshClaims.UserID

	// For now, we'll use basic info. In production, you might want to
	// fetch fresh user data from database to ensure user still exists and is active
	userEmail := ""             // Fetch from DB
	userRole := "authenticated" // Fetch from DB or use default

	// Generate new token pair
	tokenPair, err := tokenManager.GenerateTokenPair(userID, userEmail, userRole)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to generate new tokens",
		})
		return
	}

	c.JSON(http.StatusOK, tokenPair)
}

// Logout handles POST /auth/logout
// Logs out the current user
func (h *OAuthHandler) Logout(c *gin.Context) {
	// For now, just return success since logout is typically handled client-side
	// In a more complete implementation, you might invalidate the token server-side
	c.JSON(http.StatusOK, gin.H{"message": "Successfully logged out"})
}

// GetUser handles GET /auth/user
// Gets the current user's information from JWT token and creates profile if needed
// Note: AuthMiddleware already validates the token, so we can use the context data
func (h *OAuthHandler) GetUser(c *gin.Context) {
	// Get user info from context (set by AuthMiddleware)
	userID, exists := auth.GetUserID(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID not found in context"})
		return
	}

	userEmail, _ := auth.GetUserEmail(c)
	userRole := c.GetString("user_role")

	// Convert context data to JWTClaims for service compatibility
	jwtClaims := &auth.JWTClaims{
		Sub:          userID,
		Email:        userEmail,
		Role:         userRole,
		UserMetadata: make(map[string]interface{}),
		AppMetadata:  make(map[string]interface{}),
	}

	// Create or update user profile in database
	profile, err := h.userService.CreateOrUpdateUserFromJWT(c.Request.Context(), jwtClaims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile: " + err.Error()})
		return
	}

	// Return user information (without sensitive data)
	userResponse := &User{
		ID:    userID,
		Email: userEmail,
		Metadata: map[string]interface{}{
			"role": userRole,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"user":    userResponse,
		"profile": profile,
	})
}

// ProviderCallback handles GET /auth/callback
// Generic callback handler for OAuth providers
func (h *OAuthHandler) ProviderCallback(c *gin.Context) {
	// Get query parameters
	accessToken := c.Query("access_token")
	refreshToken := c.Query("refresh_token")
	errorParam := c.Query("error")
	errorDescription := c.Query("error_description")

	// Check for errors
	if errorParam != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":             errorParam,
			"error_description": errorDescription,
		})
		return
	}

	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token is required"})
		return
	}

	// Parse state parameter to get redirect URL
	redirectURL := os.Getenv("FRONTEND_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:5173"
	}

	// Redirect to frontend with tokens as URL fragments (for security)
	redirectURL = fmt.Sprintf("%s#access_token=%s&token_type=bearer",
		redirectURL,
		accessToken,
	)

	if refreshToken != "" {
		redirectURL += "&refresh_token=" + refreshToken
	}

	c.Redirect(http.StatusFound, redirectURL)
}
