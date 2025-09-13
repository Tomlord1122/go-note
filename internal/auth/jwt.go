package auth

// JWTClaims represents the structure of a Supabase JWT token
// This struct is used for service compatibility and data transfer
type JWTClaims struct {
	Sub          string                 `json:"sub"`
	Email        string                 `json:"email"`
	Role         string                 `json:"role"`
	UserMetadata map[string]interface{} `json:"user_metadata"`
	AppMetadata  map[string]interface{} `json:"app_metadata"`
	Exp          int64                  `json:"exp"`
	Iat          int64                  `json:"iat"`
}

// Note: ParseJWTClaims and IsTokenExpired functions have been removed
// Use auth.ValidateJWTToken() instead for secure JWT validation with signature verification
