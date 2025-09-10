package services

import (
	"context"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	_ "github.com/joho/godotenv/autoload"
	"google.golang.org/api/option"
)

// EmbeddingService handles text embedding operations using Google AI
type EmbeddingService struct {
	client *genai.Client
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(ctx context.Context) (*EmbeddingService, error) {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GOOGLE_API_KEY environment variable is required")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &EmbeddingService{
		client: client,
	}, nil
}

// Close closes the embedding service client
func (s *EmbeddingService) Close() error {
	return s.client.Close()
}

// GenerateEmbedding generates an embedding vector for the given text
// Uses the text-embedding-004 model which produces 768-dimensional vectors
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	if text == "" {
		return nil, fmt.Errorf("text cannot be empty")
	}

	// Use the text-embedding-004 model for consistent 768-dimensional embeddings
	em := s.client.EmbeddingModel("text-embedding-004")

	res, err := em.EmbedContent(ctx, genai.Text(text))
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	if res.Embedding == nil || len(res.Embedding.Values) == 0 {
		return nil, fmt.Errorf("empty embedding response")
	}

	return res.Embedding.Values, nil
}

// GenerateNoteEmbedding generates an embedding for a note by combining title and content
func (s *EmbeddingService) GenerateNoteEmbedding(ctx context.Context, title, content string) ([]float32, error) {
	// Combine title and content with proper formatting for better embeddings
	combinedText := fmt.Sprintf("Title: %s\n\nContent: %s", title, content)
	return s.GenerateEmbedding(ctx, combinedText)
}

// GenerateQueryEmbedding generates an embedding for a search query
func (s *EmbeddingService) GenerateQueryEmbedding(ctx context.Context, query string) ([]float32, error) {
	return s.GenerateEmbedding(ctx, query)
}
