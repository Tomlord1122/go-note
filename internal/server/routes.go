package server

import (
	"log"
	"net/http"
	"os"

	"go-note/internal/auth"
	"go-note/internal/handlers"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/joho/godotenv/autoload"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", os.Getenv("FRONTEND_URL")}, // Add your frontend URL
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true, // Enable cookies/auth
	}))

	// Initialize handlers
	userHandler := handlers.NewUserHandler(s.db.GetPool())

	// Create notes handler with services (handle nil services gracefully)
	var notesHandler *handlers.NotesHandler
	if s.embeddingService != nil && s.flashcardService != nil {
		notesHandler = handlers.NewNotesHandler(s.db.GetPool(), s.embeddingService, s.flashcardService)
	} else {
		log.Printf("Warning: Some services are not available, notes handler will have limited functionality")
		// For now, we'll create a basic handler without the services
		// You might want to create a fallback version of NewNotesHandler
		log.Fatal("Embedding and flashcard services are required for notes handler")
	}

	oauthHandler, err := handlers.NewOAuthHandler(s.db.GetPool())
	if err != nil {
		log.Fatal("Failed to create OAuth handler:", err)
	}

	// Public routes
	r.GET("/", s.HelloWorldHandler)
	r.GET("/health", s.healthHandler)

	// Authentication routes (no auth required)
	authRoutes := r.Group("/auth")
	{
		authRoutes.POST("/google/login", oauthHandler.GoogleLogin)
		authRoutes.GET("/google/callback", oauthHandler.GoogleCallback)
		authRoutes.GET("/callback", oauthHandler.ProviderCallback)
		authRoutes.POST("/refresh", oauthHandler.RefreshToken)
		authRoutes.POST("/logout", oauthHandler.Logout)
		authRoutes.GET("/user", auth.AuthMiddleware(), oauthHandler.GetUser)
	}

	// API routes
	api := r.Group("/api")
	{
		// User routes
		users := api.Group("/users")
		{
			// Public user routes (no auth required)
			users.GET("/:username", userHandler.GetUserProfileByUsername)
			users.GET("", userHandler.ListUserProfiles)

			// Protected user routes (auth required)
			protected := users.Group("", auth.AuthMiddleware())
			{
				protected.GET("/profile", userHandler.GetUserProfile)
				protected.POST("/profile", userHandler.CreateUserProfile)
				protected.PUT("/profile", userHandler.UpdateUserProfile)
				protected.DELETE("/profile", userHandler.DeleteUserProfile)
			}
		}

		// Notes routes
		notes := api.Group("/notes")
		{
			// Public notes routes (no auth required)
			notes.GET("/public", notesHandler.GetPublicNotes)
			notes.GET("/:id", auth.OptionalAuthMiddleware(), notesHandler.GetNote)

			// Protected notes routes (auth required)
			protected := notes.Group("", auth.AuthMiddleware())
			{
				protected.GET("", notesHandler.GetUserNotes)
				protected.POST("", notesHandler.CreateNote)
				protected.PUT("/:id", notesHandler.UpdateNote)
				protected.DELETE("/:id", notesHandler.DeleteNote)

				// Semantic search endpoint
				protected.POST("/search", notesHandler.SearchNotesByQuery)

				// Flashcard generation endpoints
				flashcard := protected.Group("/flashcard")
				{
					flashcard.POST("/query", notesHandler.StreamFlashcardFromQuery)
					flashcard.POST("/notes", notesHandler.StreamFlashcardFromNotes)
				}
			}
		}
	}

	return r
}

func (s *Server) HelloWorldHandler(c *gin.Context) {
	resp := make(map[string]string)
	resp["message"] = "Hello World"

	c.JSON(http.StatusOK, resp)
}

func (s *Server) healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, s.db.Health())
}
