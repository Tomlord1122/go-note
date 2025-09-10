package server

import (
	"context"
	"fmt"
	"go-note/internal/database"
	"go-note/internal/services"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/joho/godotenv/autoload"
)

type Server struct {
	port             int
	db               database.Service
	embeddingService *services.EmbeddingService
	flashcardService *services.FlashcardService
}

func NewServer() *http.Server {
	port, _ := strconv.Atoi(os.Getenv("PORT"))

	// Initialize services
	ctx := context.Background()

	embeddingService, err := services.NewEmbeddingService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize embedding service: %v", err)
		// Continue without embedding service for now
	}

	flashcardService, err := services.NewFlashcardService(ctx)
	if err != nil {
		log.Printf("Warning: Failed to initialize flashcard service: %v", err)
		// Continue without flashcard service for now
	}

	NewServer := &Server{
		port:             port,
		db:               database.New(),
		embeddingService: embeddingService,
		flashcardService: flashcardService,
	}

	// Declare Server config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", NewServer.port),
		Handler:      NewServer.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	return server
}
