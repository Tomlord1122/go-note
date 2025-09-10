package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"go-note/internal/auth"
	"go-note/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
)

// OAuthHandler handles OAuth-related HTTP requests
type OAuthHandler struct {
	supabaseClient *auth.SupabaseClient
	userService    *services.UserService
}

// NewOAuthHandler creates a new OAuth handler
func NewOAuthHandler(db *pgxpool.Pool) (*OAuthHandler, error) {
	client, err := auth.NewSupabaseClient()
	if err != nil {
		return nil, err
	}

	return &OAuthHandler{
		supabaseClient: client,
		userService:    services.NewUserService(db),
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
	claims, err := auth.ParseJWTClaims(accessToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token format: " + err.Error()})
		return
	}

	userResponse := &User{
		ID:       claims.Sub,
		Email:    claims.Email,
		Metadata: claims.UserMetadata,
	}

	c.JSON(http.StatusOK, AuthResponse{
		AccessToken: accessToken,
		User:        userResponse,
		Message:     "Successfully authenticated",
	})
}

// RefreshToken handles POST /auth/refresh
// Refreshes an expired access token using the refresh token
func (h *OAuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Refresh token is required"})
		return
	}

	// Use the refresh token to get a new access token
	// This would typically involve a call to Supabase's refresh endpoint
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "Refresh token functionality not implemented yet",
		"message": "Please re-authenticate using the login flow",
	})
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
func (h *OAuthHandler) GetUser(c *gin.Context) {
	// Get token from Authorization header
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return
	}

	// Extract token from "Bearer <token>"
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		return
	}

	// Parse JWT token to extract user information
	claims, err := auth.ParseJWTClaims(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format: " + err.Error()})
		return
	}

	// Check if token is expired
	if auth.IsTokenExpired(claims) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
		return
	}

	// Create or update user profile in database
	profile, err := h.userService.CreateOrUpdateUserFromJWT(c.Request.Context(), claims)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile: " + err.Error()})
		return
	}

	// Return user information (without sensitive data)
	userResponse := &User{
		ID:    claims.Sub,
		Email: claims.Email,
		// Don't include full metadata for security
		Metadata: map[string]interface{}{
			"full_name": claims.UserMetadata["full_name"],
			"picture":   claims.UserMetadata["picture"],
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
