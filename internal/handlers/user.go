package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"go-note/internal/auth"
	db_sqlc "go-note/internal/db_sqlc"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserHandler handles user-related HTTP requests
type UserHandler struct {
	queries *db_sqlc.Queries
	db      *pgxpool.Pool
}

// NewUserHandler creates a new user handler
func NewUserHandler(db *pgxpool.Pool) *UserHandler {
	return &UserHandler{
		queries: db_sqlc.New(db),
		db:      db,
	}
}

// CreateUserProfileRequest represents the request body for creating a user profile
type CreateUserProfileRequest struct {
	Username    *string                `json:"username,omitempty"`
	DisplayName *string                `json:"display_name,omitempty"`
	AvatarURL   *string                `json:"avatar_url,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
}

// UpdateUserProfileRequest represents the request body for updating a user profile
type UpdateUserProfileRequest struct {
	Username    *string                `json:"username,omitempty"`
	DisplayName *string                `json:"display_name,omitempty"`
	AvatarURL   *string                `json:"avatar_url,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
}

// UserProfileResponse represents the response format for user profiles
type UserProfileResponse struct {
	ID          string                 `json:"id"`
	Username    *string                `json:"username,omitempty"`
	DisplayName *string                `json:"display_name,omitempty"`
	AvatarURL   *string                `json:"avatar_url,omitempty"`
	Preferences map[string]interface{} `json:"preferences,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	UpdatedAt   string                 `json:"updated_at"`
}

// GetUserProfile handles GET /api/users/profile
func (h *UserHandler) GetUserProfile(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	// Parse UUID
	var uuid pgtype.UUID
	if err := uuid.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	profile, err := h.queries.GetUserProfile(c.Request.Context(), uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}

	response := convertUserProfileToResponse(profile)
	c.JSON(http.StatusOK, response)
}

// GetUserProfileByUsername handles GET /api/users/:username
func (h *UserHandler) GetUserProfileByUsername(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Username is required"})
		return
	}

	var usernameText pgtype.Text
	usernameText.Scan(username)

	profile, err := h.queries.GetUserProfileByUsername(c.Request.Context(), usernameText)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User profile not found"})
		return
	}

	response := convertUserProfileToResponse(profile)
	c.JSON(http.StatusOK, response)
}

// CreateUserProfile handles POST /api/users/profile
func (h *UserHandler) CreateUserProfile(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req CreateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if username is already taken
	if req.Username != nil {
		var usernameText pgtype.Text
		usernameText.Scan(*req.Username)

		exists, err := h.queries.CheckUsernameExists(c.Request.Context(), usernameText)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if exists {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
			return
		}
	}

	// Parse UUID
	var uuid pgtype.UUID
	if err := uuid.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Prepare parameters
	params := db_sqlc.CreateUserProfileParams{
		ID: uuid,
	}

	if req.Username != nil {
		params.Username.Scan(*req.Username)
	}
	if req.DisplayName != nil {
		params.DisplayName.Scan(*req.DisplayName)
	}
	if req.AvatarURL != nil {
		params.AvatarUrl.Scan(*req.AvatarURL)
	}
	if req.Preferences != nil {
		preferencesJSON, err := json.Marshal(req.Preferences)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid preferences format"})
			return
		}
		params.Preferences = preferencesJSON
	}

	profile, err := h.queries.CreateUserProfile(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user profile"})
		return
	}

	response := convertUserProfileToResponse(profile)
	c.JSON(http.StatusCreated, response)
}

// UpdateUserProfile handles PUT /api/users/profile
func (h *UserHandler) UpdateUserProfile(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Check if username is already taken by another user
	if req.Username != nil {
		var usernameText pgtype.Text
		usernameText.Scan(*req.Username)

		exists, err := h.queries.CheckUsernameExists(c.Request.Context(), usernameText)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
		if exists {
			// Check if it's the current user's username
			var uuid pgtype.UUID
			uuid.Scan(userID)
			currentProfile, err := h.queries.GetUserProfile(c.Request.Context(), uuid)
			if err == nil && currentProfile.Username.String != *req.Username {
				c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
				return
			}
		}
	}

	// Parse UUID
	var uuid pgtype.UUID
	if err := uuid.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Prepare parameters
	params := db_sqlc.UpdateUserProfileParams{
		ID: uuid,
	}

	if req.Username != nil {
		params.Username.Scan(*req.Username)
	}
	if req.DisplayName != nil {
		params.DisplayName.Scan(*req.DisplayName)
	}
	if req.AvatarURL != nil {
		params.AvatarUrl.Scan(*req.AvatarURL)
	}
	if req.Preferences != nil {
		preferencesJSON, err := json.Marshal(req.Preferences)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid preferences format"})
			return
		}
		params.Preferences = preferencesJSON
	}

	profile, err := h.queries.UpdateUserProfile(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user profile"})
		return
	}

	response := convertUserProfileToResponse(profile)
	c.JSON(http.StatusOK, response)
}

// DeleteUserProfile handles DELETE /api/users/profile
func (h *UserHandler) DeleteUserProfile(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	// Parse UUID
	var uuid pgtype.UUID
	if err := uuid.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	err := h.queries.DeleteUserProfile(c.Request.Context(), uuid)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user profile"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListUserProfiles handles GET /api/users
func (h *UserHandler) ListUserProfiles(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 || limit > 100 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	params := db_sqlc.ListUserProfilesParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	}

	profiles, err := h.queries.ListUserProfiles(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user profiles"})
		return
	}

	var responses []UserProfileResponse
	for _, profile := range profiles {
		responses = append(responses, convertUserProfileToResponse(profile))
	}

	c.JSON(http.StatusOK, gin.H{
		"users":  responses,
		"limit":  limit,
		"offset": offset,
		"count":  len(responses),
	})
}

// convertUserProfileToResponse converts a database UserProfile to API response format
func convertUserProfileToResponse(profile db_sqlc.UserProfile) UserProfileResponse {
	response := UserProfileResponse{
		ID:        profile.ID.String(),
		CreatedAt: profile.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: profile.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}

	if profile.Username.Valid {
		response.Username = &profile.Username.String
	}
	if profile.DisplayName.Valid {
		response.DisplayName = &profile.DisplayName.String
	}
	if profile.AvatarUrl.Valid {
		response.AvatarURL = &profile.AvatarUrl.String
	}
	if len(profile.Preferences) > 0 {
		var preferences map[string]interface{}
		if err := json.Unmarshal(profile.Preferences, &preferences); err == nil {
			response.Preferences = preferences
		}
	}

	return response
}
