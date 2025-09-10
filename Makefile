# Simple Makefile for a Go project

# Build the application
all: build test

build: sqlc-generate
	@echo "Building..."
	@go build -o main cmd/api/main.go

# Run the application
run:
	@go run cmd/api/main.go
# Install SQLC
sqlc-install:
	@if ! command -v sqlc > /dev/null; then \
		read -p "SQLC is not installed. Do you want to install it? [Y/n] " choice; \
		if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
			go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest; \
			if [ ! -x "$$(command -v sqlc)" ]; then \
				echo "SQLC installation failed. Exiting..."; \
				exit 1; \
			fi; \
		else \
			echo "You chose not to install SQLC. Exiting..."; \
			exit 1; \
		fi; \
	fi

# Generate SQLC code
sqlc-generate: sqlc-install
	@echo "Generating SQLC code..."
	@sqlc generate

# Verify SQLC queries
sqlc-verify: sqlc-install
	@echo "Verifying SQLC queries..."
	@sqlc verify
# Create DB container
docker-run:
	@if docker compose up --build 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose up --build; \
	fi

# Shutdown DB container
docker-down:
	@if docker compose down 2>/dev/null; then \
		: ; \
	else \
		echo "Falling back to Docker Compose V1"; \
		docker-compose down; \
	fi

# Test the application
test:
	@echo "Testing..."
	@go test ./... -v
# Integrations Tests for the application
itest:
	@echo "Running integration tests..."
	@go test ./internal/database -v

# Clean the binary
clean:
	@echo "Cleaning..."
	@rm -f main

# Live Reload
watch:
	@if command -v air > /dev/null; then \
            air; \
            echo "Watching...";\
        else \
            read -p "Go's 'air' is not installed on your machine. Do you want to install it? [Y/n] " choice; \
            if [ "$$choice" != "n" ] && [ "$$choice" != "N" ]; then \
                go install github.com/air-verse/air@latest; \
                air; \
                echo "Watching...";\
            else \
                echo "You chose not to install air. Exiting..."; \
                exit 1; \
            fi; \
        fi

# Database commands
db-start:
	@echo "Starting Supabase local development..."
	supabase start

db-stop:
	@echo "Stopping Supabase local development..."
	supabase stop

db-up:
	@echo "Running database migrations..."
	supabase migration up

# Database migration commands
db-schema:
	@if [ -z "$(name)" ]; then \
		echo "Error: Migration name is required. Usage: make db-schema name=your_migration_name"; \
		exit 1; \
	fi
	@echo "Creating new migration: $(name)"
	@migrate create -ext sql -dir ./supabase/migrations $(name)

.PHONY: all build run test clean watch docker-run docker-down itest sqlc-install sqlc-generate sqlc-verify db-start db-stop db-up db-schema
