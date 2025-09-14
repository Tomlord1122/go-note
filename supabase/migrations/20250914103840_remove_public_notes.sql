-- Remove public note functionality from the notes table

-- Drop the function that references is_public column
DROP FUNCTION IF EXISTS search_notes_by_similarity(vector, float, int, uuid);

-- Drop RLS policies that reference is_public column
DROP POLICY IF EXISTS "Users can view own notes or public notes" ON notes;

-- Remove is_public column from notes table
ALTER TABLE notes DROP COLUMN IF EXISTS is_public;

-- Recreate RLS policy without public note access
CREATE POLICY "Users can view own notes" ON notes
    FOR SELECT USING (auth.uid() = user_id);

-- Recreate the search function without public note support
CREATE OR REPLACE FUNCTION search_notes_by_similarity(
    query_embedding VECTOR(768),
    match_threshold FLOAT DEFAULT 0.7,
    match_count INT DEFAULT 10,
    target_user_id UUID DEFAULT NULL
)
RETURNS TABLE (
    id UUID,
    title VARCHAR(500),
    content TEXT,
    tags TEXT[],
    similarity FLOAT,
    created_at TIMESTAMP WITH TIME ZONE
)
LANGUAGE plpgsql
AS $$
BEGIN
    RETURN QUERY
    SELECT 
        n.id,
        n.title,
        n.content,
        n.tags,
        1 - (n.embedding <=> query_embedding) AS similarity,
        n.created_at
    FROM notes n
    WHERE 
        (target_user_id IS NULL OR n.user_id = target_user_id)
        AND n.embedding IS NOT NULL
        AND 1 - (n.embedding <=> query_embedding) > match_threshold
    ORDER BY n.embedding <=> query_embedding
    LIMIT match_count;
END;
$$;
