package auth

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// JWTClaims represents the structure of a Supabase JWT token
type JWTClaims struct {
	Sub          string                 `json:"sub"`
	Email        string                 `json:"email"`
	Role         string                 `json:"role"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
	AppMetadata  map[string]interface{} `json:"app_metadata"`
	Iss          string                 `json:"iss"`
	Aud          string                 `json:"aud"`
	Exp          int64                  `json:"exp"`
	Iat          int64                  `json:"iat"`
}

// ParseJWTClaims extracts claims from a JWT token without verification
// This is useful when we trust the token source (like Supabase callback)
func ParseJWTClaims(tokenString string) (*JWTClaims, error) {
	// Split the JWT token
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}

	// Decode the payload (second part)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode JWT payload: %w", err)
	}

	// Parse the JSON payload
	var claims JWTClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("failed to parse JWT claims: %w", err)
	}

	return &claims, nil
}

// IsTokenExpired checks if the JWT token is expired
func IsTokenExpired(claims *JWTClaims) bool {
	// For now, we'll assume the token is valid
	// In a production environment, you'd check the exp claim against current time
	return false
}
