package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"

	"go-note/internal/auth"
	db_sqlc "go-note/internal/db_sqlc"
	"go-note/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

// NotesHandler handles note-related HTTP requests
type NotesHandler struct {
	queries          *db_sqlc.Queries
	db               *pgxpool.Pool
	embeddingService *services.EmbeddingService
	flashcardService *services.FlashcardService
}

// NewNotesHandler creates a new notes handler
func NewNotesHandler(db *pgxpool.Pool, embeddingService *services.EmbeddingService, flashcardService *services.FlashcardService) *NotesHandler {
	return &NotesHandler{
		queries:          db_sqlc.New(db),
		db:               db,
		embeddingService: embeddingService,
		flashcardService: flashcardService,
	}
}

// CreateNoteRequest represents the request body for creating a note
type CreateNoteRequest struct {
	Title   string   `json:"title" binding:"required"`
	Content string   `json:"content" binding:"required"`
	Tags    []string `json:"tags,omitempty"`
}

// UpdateNoteRequest represents the request body for updating a note
type UpdateNoteRequest struct {
	Title   *string  `json:"title,omitempty"`
	Content *string  `json:"content,omitempty"`
	Tags    []string `json:"tags,omitempty"`
}

// NoteResponse represents the response format for notes
type NoteResponse struct {
	ID        string   `json:"id"`
	UserID    string   `json:"user_id"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// CreateNote handles POST /api/notes
func (h *NotesHandler) CreateNote(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse user UUID
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Generate embedding for the note
	embedding, err := h.embeddingService.GenerateNoteEmbedding(c.Request.Context(), req.Title, req.Content)
	if err != nil {
		log.Printf("Failed to generate embedding: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate embedding"})
		return
	}

	// Convert embedding to pgvector.Vector
	embeddingVector := pgvector.NewVector(embedding)

	// Prepare parameters
	params := db_sqlc.CreateNoteParams{
		UserID:    userUUID,
		Title:     req.Title,
		Content:   req.Content,
		Embedding: embeddingVector,
		Tags:      req.Tags,
	}

	// Create the note
	note, err := h.queries.CreateNote(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create note: " + err.Error()})
		return
	}

	response := convertCreateNoteRowToResponse(note)
	c.JSON(http.StatusCreated, response)
}

// GetNote handles GET /api/notes/:id
func (h *NotesHandler) GetNote(c *gin.Context) {
	noteIDStr := c.Param("id")
	if noteIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Note ID is required"})
		return
	}

	// Parse note UUID
	var noteUUID pgtype.UUID
	if err := noteUUID.Scan(noteIDStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID format"})
		return
	}

	// Get the note
	note, err := h.queries.GetNote(c.Request.Context(), noteUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		return
	}

	// Check if user can access this note
	userID, authenticated := auth.GetUserID(c)
	if !authenticated || userID != note.UserID.String() {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	response := convertGetNoteRowToResponse(note)
	c.JSON(http.StatusOK, response)
}

// GetUserNotes handles GET /api/notes
func (h *NotesHandler) GetUserNotes(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

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

	// Parse user UUID
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Get user's notes
	notes, err := h.queries.GetUserNotes(c.Request.Context(), db_sqlc.GetUserNotesParams{
		UserID: userUUID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch notes"})
		return
	}

	var responses []NoteResponse
	for _, note := range notes {
		responses = append(responses, convertGetUserNotesRowToResponse(note))
	}

	c.JSON(http.StatusOK, gin.H{
		"notes":  responses,
		"limit":  limit,
		"offset": offset,
		"count":  len(responses),
	})
}

// UpdateNote handles PUT /api/notes/:id
func (h *NotesHandler) UpdateNote(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	noteIDStr := c.Param("id")
	if noteIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Note ID is required"})
		return
	}

	var req UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse UUIDs
	var noteUUID, userUUID pgtype.UUID
	if err := noteUUID.Scan(noteIDStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID format"})
		return
	}
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// First get the current note to check if title/content changed
	currentNote, err := h.queries.GetNote(c.Request.Context(), noteUUID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Note not found"})
		return
	}

	// Check if user owns the note
	if currentNote.UserID.String() != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Prepare parameters
	params := db_sqlc.UpdateNoteParams{
		ID:     noteUUID,
		UserID: userUUID,
	}

	// Check if we need to regenerate embedding
	needsEmbeddingUpdate := false
	newTitle := currentNote.Title
	newContent := currentNote.Content

	if req.Title != nil {
		params.Title = *req.Title
		newTitle = *req.Title
		needsEmbeddingUpdate = true
	}
	if req.Content != nil {
		params.Content = *req.Content
		newContent = *req.Content
		needsEmbeddingUpdate = true
	}
	if req.Tags != nil {
		params.Tags = req.Tags
	}

	// Generate new embedding if title or content changed
	if needsEmbeddingUpdate {
		embedding, err := h.embeddingService.GenerateNoteEmbedding(c.Request.Context(), newTitle, newContent)
		if err != nil {
			log.Printf("Failed to generate embedding: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate embedding"})
			return
		}
		params.Embedding = pgvector.NewVector(embedding)
	}

	// Update the note
	note, err := h.queries.UpdateNote(c.Request.Context(), params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update note: " + err.Error()})
		return
	}

	response := convertUpdateNoteRowToResponse(note)
	c.JSON(http.StatusOK, response)
}

// DeleteNote handles DELETE /api/notes/:id
func (h *NotesHandler) DeleteNote(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	noteIDStr := c.Param("id")
	if noteIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Note ID is required"})
		return
	}

	// Parse UUIDs
	var noteUUID, userUUID pgtype.UUID
	if err := noteUUID.Scan(noteIDStr); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID format"})
		return
	}
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Delete the note
	err := h.queries.DeleteNote(c.Request.Context(), db_sqlc.DeleteNoteParams{
		ID:     noteUUID,
		UserID: userUUID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete note"})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// convertCreateNoteRowToResponse converts CreateNoteRow to API response format
func convertCreateNoteRowToResponse(note db_sqlc.CreateNoteRow) NoteResponse {
	return NoteResponse{
		ID:        note.ID.String(),
		UserID:    note.UserID.String(),
		Title:     note.Title,
		Content:   note.Content,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: note.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// convertGetNoteRowToResponse converts GetNoteRow to API response format
func convertGetNoteRowToResponse(note db_sqlc.GetNoteRow) NoteResponse {
	return NoteResponse{
		ID:        note.ID.String(),
		UserID:    note.UserID.String(),
		Title:     note.Title,
		Content:   note.Content,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: note.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// convertGetUserNotesRowToResponse converts GetUserNotesRow to API response format
func convertGetUserNotesRowToResponse(note db_sqlc.GetUserNotesRow) NoteResponse {
	return NoteResponse{
		ID:        note.ID.String(),
		UserID:    note.UserID.String(),
		Title:     note.Title,
		Content:   note.Content,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: note.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// convertUpdateNoteRowToResponse converts UpdateNoteRow to API response format
func convertUpdateNoteRowToResponse(note db_sqlc.UpdateNoteRow) NoteResponse {
	return NoteResponse{
		ID:        note.ID.String(),
		UserID:    note.UserID.String(),
		Title:     note.Title,
		Content:   note.Content,
		Tags:      note.Tags,
		CreatedAt: note.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: note.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// SearchNotesRequest represents the request for searching notes by query
type SearchNotesRequest struct {
	Query     string  `json:"query" binding:"required"`
	Threshold float64 `json:"threshold,omitempty"`
	Limit     int     `json:"limit,omitempty"`
}

// GenerateFlashcardFromQueryRequest represents the request for generating flashcard from query
type GenerateFlashcardFromQueryRequest struct {
	Query string `json:"query" binding:"required"`
}

// GenerateFlashcardFromNotesRequest represents the request for generating flashcard from selected notes
type GenerateFlashcardFromNotesRequest struct {
	NoteIDs []string `json:"note_ids" binding:"required,min=1"`
}

// SearchNotesByQuery handles POST /api/notes/search
// 方案1: 根據查詢搜尋相關筆記
func (h *NotesHandler) SearchNotesByQuery(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req SearchNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Set default values
	if req.Threshold == 0 {
		req.Threshold = 0.7
	}
	if req.Limit == 0 {
		req.Limit = 10
	}

	// Generate embedding for the query
	queryEmbedding, err := h.embeddingService.GenerateQueryEmbedding(c.Request.Context(), req.Query)
	if err != nil {
		log.Printf("Failed to generate query embedding: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process query"})
		return
	}

	// Parse user UUID
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Search for similar notes
	notes, err := h.queries.SearchNotesBySimilarity(c.Request.Context(), db_sqlc.SearchNotesBySimilarityParams{
		Column1: pgvector.NewVector(queryEmbedding), // Query embedding
		UserID:  userUUID,
		Column3: req.Threshold, // Similarity threshold
		Limit:   int32(req.Limit),
	})
	if err != nil {
		log.Printf("Failed to search notes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search notes"})
		return
	}

	// Convert to response format
	var responses []map[string]interface{}
	for _, note := range notes {
		responses = append(responses, map[string]interface{}{
			"id":         note.ID.String(),
			"user_id":    note.UserID.String(),
			"title":      note.Title,
			"content":    note.Content,
			"tags":       note.Tags,
			"created_at": note.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			"updated_at": note.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
			"similarity": note.Similarity,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"notes":   responses,
		"count":   len(responses),
		"results": len(responses),
	})
}

// StreamFlashcardFromQuery handles POST /api/notes/flashcard/query
func (h *NotesHandler) StreamFlashcardFromQuery(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req GenerateFlashcardFromQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Generate embedding for the query
	queryEmbedding, err := h.embeddingService.GenerateQueryEmbedding(c.Request.Context(), req.Query)
	if err != nil {
		log.Printf("Failed to generate query embedding: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process query"})
		return
	}

	// Parse user UUID
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Search for similar notes
	notes, err := h.queries.SearchNotesBySimilarity(c.Request.Context(), db_sqlc.SearchNotesBySimilarityParams{
		Column1: pgvector.NewVector(queryEmbedding),
		UserID:  userUUID,
		Column3: 0.6,
		Limit:   5,
	})
	if err != nil {
		log.Printf("Failed to search notes: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to search notes"})
		return
	}

	if len(notes) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "No relevant notes found for the query"})
		return
	}

	// Convert to service format
	var serviceNotes []services.Note
	for _, note := range notes {
		serviceNotes = append(serviceNotes, services.Note{
			ID:      note.ID.String(),
			Title:   note.Title,
			Content: note.Content,
			Tags:    note.Tags,
		})
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	responseChan := make(chan string, 100)

	go func() {
		_ = h.flashcardService.StreamFlashcardFromQuery(c.Request.Context(), req.Query, serviceNotes, responseChan)
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case message, ok := <-responseChan:
			if !ok {
				return false
			}
			_, _ = w.Write([]byte(message))
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// StreamFlashcardFromNotes handles POST /api/notes/flashcard/notes/stream
func (h *NotesHandler) StreamFlashcardFromNotes(c *gin.Context) {
	userID, exists := auth.RequireAuth(c)
	if !exists {
		return
	}

	var req GenerateFlashcardFromNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body: " + err.Error()})
		return
	}

	// Parse user UUID
	var userUUID pgtype.UUID
	if err := userUUID.Scan(userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID format"})
		return
	}

	// Fetch the selected notes
	var serviceNotes []services.Note
	for _, noteIDStr := range req.NoteIDs {
		var noteUUID pgtype.UUID
		if err := noteUUID.Scan(noteIDStr); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid note ID format: " + noteIDStr})
			return
		}

		note, err := h.queries.GetNoteForFlashcard(c.Request.Context(), db_sqlc.GetNoteForFlashcardParams{
			ID:     noteUUID,
			UserID: userUUID,
		})
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Note not found or access denied: " + noteIDStr})
			return
		}

		serviceNotes = append(serviceNotes, services.Note{
			ID:      note.ID.String(),
			Title:   note.Title,
			Content: note.Content,
			Tags:    note.Tags,
		})
	}

	if len(serviceNotes) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No valid notes found"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	responseChan := make(chan string, 100)

	go func() {
		_ = h.flashcardService.StreamFlashcardFromNotes(c.Request.Context(), serviceNotes, responseChan)
	}()

	c.Stream(func(w io.Writer) bool {
		select {
		case message, ok := <-responseChan:
			if !ok {
				return false
			}
			_, _ = w.Write([]byte(message))
			if f, ok := c.Writer.(http.Flusher); ok {
				f.Flush()
			}
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}
