package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"go-note/internal/auth"
	db_sqlc "go-note/internal/db_sqlc"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// UserService handles user-related business logic
type UserService struct {
	queries *db_sqlc.Queries
	db      *pgxpool.Pool
}

// NewUserService creates a new user service
func NewUserService(db *pgxpool.Pool) *UserService {
	return &UserService{
		queries: db_sqlc.New(db),
		db:      db,
	}
}

// CreateOrUpdateUserFromJWT creates or updates a user profile from JWT claims
func (s *UserService) CreateOrUpdateUserFromJWT(ctx context.Context, claims *auth.JWTClaims) (*db_sqlc.UserProfile, error) {
	// Parse UUID - Supabase UUIDs are in string format
	var userUUID pgtype.UUID
	if err := userUUID.Scan(claims.Sub); err != nil {
		log.Printf("Failed to parse user UUID: %s, error: %v", claims.Sub, err)
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	// Check if user profile already exists
	existingProfile, err := s.queries.GetUserProfile(ctx, userUUID)
	if err == nil {
		// User exists, return existing profile
		log.Printf("User profile already exists for user: %s", claims.Email)
		return &existingProfile, nil
	}

	// User doesn't exist, create new profile
	log.Printf("Creating new user profile for: %s", claims.Email)

	// Generate username from email (before @ symbol)
	username := claims.Email
	if atIndex := len(claims.Email); atIndex > 0 {
		for i, c := range claims.Email {
			if c == '@' {
				username = claims.Email[:i]
				break
			}
		}
	}

	// Get display name from metadata
	displayName := claims.Email
	if fullName, ok := claims.UserMetadata["full_name"].(string); ok && fullName != "" {
		displayName = fullName
	} else if name, ok := claims.UserMetadata["name"].(string); ok && name != "" {
		displayName = name
	}

	// Get avatar URL from metadata
	avatarURL := ""
	if picture, ok := claims.UserMetadata["picture"].(string); ok {
		avatarURL = picture
	} else if avatarUrl, ok := claims.UserMetadata["avatar_url"].(string); ok {
		avatarURL = avatarUrl
	}

	// Create default preferences
	defaultPreferences := map[string]interface{}{
		"theme":         "light",
		"notifications": true,
		"language":      "zh-TW",
	}
	preferencesJSON, _ := json.Marshal(defaultPreferences)

	// Create user profile parameters
	params := db_sqlc.CreateUserProfileParams{
		ID: userUUID,
	}

	// Set username (ensure uniqueness)
	uniqueUsername, err := s.generateUniqueUsername(ctx, username)
	if err != nil {
		return nil, err
	}
	params.Username.Scan(uniqueUsername)

	// Set other fields
	params.DisplayName.Scan(displayName)
	if avatarURL != "" {
		params.AvatarUrl.Scan(avatarURL)
	}
	params.Preferences = preferencesJSON

	// Create the user profile
	profile, err := s.queries.CreateUserProfile(ctx, params)
	if err != nil {
		log.Printf("Failed to create user profile: %v", err)
		return nil, err
	}

	log.Printf("Successfully created user profile: %s (%s)", profile.DisplayName.String, profile.Username.String)
	return &profile, nil
}

// generateUniqueUsername generates a unique username by checking existing usernames
func (s *UserService) generateUniqueUsername(ctx context.Context, baseUsername string) (string, error) {
	// Clean the base username (remove special characters, make lowercase)
	cleanUsername := baseUsername

	// First try the base username
	var usernameText pgtype.Text
	usernameText.Scan(cleanUsername)

	exists, err := s.queries.CheckUsernameExists(ctx, usernameText)
	if err != nil {
		return "", err
	}

	if !exists {
		return cleanUsername, nil
	}

	// If base username exists, try with numbers
	for i := 1; i <= 999; i++ {
		candidateUsername := cleanUsername + "_" + fmt.Sprintf("%d", i)
		usernameText.Scan(candidateUsername)

		exists, err := s.queries.CheckUsernameExists(ctx, usernameText)
		if err != nil {
			return "", err
		}

		if !exists {
			return candidateUsername, nil
		}
	}

	// If all numbered versions exist, use timestamp
	candidateUsername := cleanUsername + "_" + fmt.Sprintf("%d", time.Now().Unix())
	return candidateUsername, nil
}

// GetUserProfile gets user profile by user ID
func (s *UserService) GetUserProfile(ctx context.Context, userID string) (*db_sqlc.UserProfile, error) {
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		return nil, fmt.Errorf("invalid user ID format: %w", err)
	}

	profile, err := s.queries.GetUserProfile(ctx, userUUID)
	if err != nil {
		return nil, err
	}

	return &profile, nil
}
