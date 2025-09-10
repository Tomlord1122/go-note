-- name: CreateNote :one
INSERT INTO notes (user_id, title, content, embedding, tags, is_public)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, title, content, tags, is_public, created_at, updated_at;

-- name: GetNote :one
SELECT id, user_id, title, content, tags, is_public, created_at, updated_at
FROM notes
WHERE id = $1;

-- name: GetUserNotes :many
SELECT id, user_id, title, content, tags, is_public, created_at, updated_at
FROM notes
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateNote :one
UPDATE notes
SET 
    title = COALESCE($2, title),
    content = COALESCE($3, content),
    embedding = COALESCE($4, embedding),
    tags = COALESCE($5, tags),
    is_public = COALESCE($6, is_public),
    updated_at = NOW()
WHERE id = $1 AND user_id = $7
RETURNING id, user_id, title, content, tags, is_public, created_at, updated_at;

-- name: DeleteNote :exec
DELETE FROM notes
WHERE id = $1 AND user_id = $2;

-- name: GetPublicNotes :many
SELECT id, user_id, title, content, tags, is_public, created_at, updated_at
FROM notes
WHERE is_public = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: SearchNotesBySimilarity :many
SELECT 
    n.id,
    n.user_id,
    n.title,
    n.content,
    n.tags,
    n.is_public,
    n.created_at,
    n.updated_at,
    (1 - (n.embedding <=> $1::vector))::float AS similarity
FROM notes n
WHERE 
    (n.user_id = $2 OR n.is_public = true)
    AND n.embedding IS NOT NULL
    AND 1 - (n.embedding <=> $1::vector) > $3::float
ORDER BY n.embedding <=> $1::vector
LIMIT $4;

-- name: GetNoteForFlashcard :one
SELECT id, user_id, title, content, tags, created_at
FROM notes
WHERE id = $1 AND (user_id = $2 OR is_public = true)
LIMIT 1;
