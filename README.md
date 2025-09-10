# Go-Note

A note-taking REST API built with Go, featuring AI-powered semantic search and flashcard generation. It is my personal project to learn Go backend with supabase and langchain.

## What is Go-Note?

Go-Note is a powerful backend API for note-taking applications that combines traditional note management with AI capabilities. It allows users to create, organize, and search through their notes using advanced semantic search powered by Google's embeddings, and can automatically generate flashcards from note content.

## Key Features

- **🔐 Google OAuth Authentication** - Secure login with Supabase integration
- **📱 User Profiles** - Customizable user profiles with avatars and preferences
- **📝 Note Management** - Create, read, update, and delete notes with tagging
- **🔍 AI-Powered Search** - Semantic search using Google embeddings and vector similarity
- **🎯 Smart Flashcards** - Auto-generate study flashcards from your notes or queries
- **🌐 Public/Private Notes** - Share notes publicly or keep them private
- **⚡ Real-time Streaming** - Server-sent events for flashcard generation
- **🔒 Row-Level Security** - Database-level security with Supabase RLS policies

## Tech Stack

- **Backend**: Go 1.25+ with Gin framework
- **Database**: PostgreSQL with pgvector for embeddings
- **Auth**: Supabase Authentication with JWT
- **AI**: Google Generative AI for embeddings and flashcard generation
- **SQL**: SQLC for type-safe database queries
- **Deployment**: Docker ready with Fly.io configuration

## API Endpoints

### Authentication
- `POST /auth/google/login` - Google OAuth login
- `GET /auth/google/callback` - OAuth callback
- `POST /auth/refresh` - Refresh JWT token
- `POST /auth/logout` - User logout

### User Management
- `GET /api/users/profile` - Get user profile
- `POST /api/users/profile` - Create user profile
- `PUT /api/users/profile` - Update user profile
- `GET /api/users/:username` - Get public user profile

### Notes
- `GET /api/notes` - Get user's notes
- `POST /api/notes` - Create new note
- `PUT /api/notes/:id` - Update note
- `DELETE /api/notes/:id` - Delete note
- `GET /api/notes/public` - Get public notes
- `POST /api/notes/search` - Semantic search through notes

### AI Features
- `POST /api/notes/flashcard/query` - Generate flashcards from query
- `POST /api/notes/flashcard/notes` - Generate flashcards from selected notes

## Quick Start

### Prerequisites
- Go 1.25+
- Docker & Docker Compose
- Supabase CLI
- Make (optional but recommended)

### Environment Setup
```bash
# Clone the repository
git clone <repository-url>
cd go-note

# Copy environment template
cp .env.example .env

# Edit .env with your configuration
# - Google OAuth credentials
# - Supabase keys
# - Database connection details
```

### Database Setup
```bash
# Start Supabase local development
make db-start

# Generate type-safe SQL code
make sqlc-generate
```

### Running the Application
```bash
# Development with live reload
make watch

# Or run once
make run

# Or build and run
make build
./main
```

### Testing
```bash
# Run all tests
make test

# Run integration tests
make itest
```

## Project Structure

```
go-note/
├── cmd/api/                 # Application entry point
├── internal/
│   ├── auth/               # JWT and authentication middleware
│   ├── database/           # Database connection service
│   ├── db_sqlc/           # Generated type-safe SQL queries
│   ├── handlers/          # HTTP request handlers
│   ├── server/            # HTTP server setup and routing
│   ├── services/          # Business logic (AI, embeddings)
│   └── utils/             # Utility functions
├── supabase/
│   ├── migrations/        # Database schema migrations
│   └── queries/           # SQL queries for SQLC
├── Makefile               # Development commands
└── Dockerfile             # Container configuration
```

## Development Commands

The project includes a comprehensive Makefile for development:

```bash
make build          # Build the application
make run            # Run the application
make watch          # Live reload development
make test           # Run unit tests
make itest          # Run integration tests
make sqlc-generate  # Generate type-safe SQL code
make db-start       # Start Supabase local
make db-stop        # Stop Supabase local
make clean          # Clean build artifacts
```

## 🌟 Architecture Highlights

- **Clean Architecture** - Separation of concerns with clear layers
- **Type Safety** - SQLC generates type-safe database queries
- **Dependency Injection** - Services injected through constructors
- **Graceful Shutdown** - Proper server shutdown handling
- **Integration Testing** - Real database tests with testcontainers
- **Vector Search** - pgvector for efficient similarity search
- **Row-Level Security** - Database-level access control




