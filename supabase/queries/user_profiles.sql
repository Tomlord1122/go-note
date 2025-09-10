-- name: GetUserProfile :one
SELECT id, username, display_name, avatar_url, preferences, created_at, updated_at
FROM user_profiles
WHERE id = $1;

-- name: GetUserProfileByUsername :one
SELECT id, username, display_name, avatar_url, preferences, created_at, updated_at
FROM user_profiles
WHERE username = $1;

-- name: CreateUserProfile :one
INSERT INTO user_profiles (id, username, display_name, avatar_url, preferences)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, username, display_name, avatar_url, preferences, created_at, updated_at;

--  COALESCE is used to update the user profile with the new values if they are not null, if they are null, the old value will be kept.
-- name: UpdateUserProfile :one
UPDATE user_profiles
SET 
    username = COALESCE($2, username),
    display_name = COALESCE($3, display_name),
    avatar_url = COALESCE($4, avatar_url),
    preferences = COALESCE($5, preferences),
    updated_at = NOW()
WHERE id = $1
RETURNING id, username, display_name, avatar_url, preferences, created_at, updated_at;

-- name: DeleteUserProfile :exec
DELETE FROM user_profiles
WHERE id = $1;

-- name: ListUserProfiles :many
SELECT id, username, display_name, avatar_url, preferences, created_at, updated_at
FROM user_profiles
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: CheckUsernameExists :one
SELECT EXISTS(
    SELECT 1 FROM user_profiles WHERE username = $1
) as exists;
