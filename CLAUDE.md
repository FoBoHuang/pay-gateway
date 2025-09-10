# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **Google Play Billing Service** - a Go-based microservice that handles in-app purchases and subscription verification for Android applications. The service integrates with Google Play's Android Developer API to verify purchases, manage subscriptions, and process webhook events.

## Technology Stack

- **Language**: Go 1.21
- **Web Framework**: Gin (v1.9.1)
- **Database**: PostgreSQL with GORM ORM
- **Cache**: Redis (go-redis/v9)
- **Authentication**: JWT (golang-jwt/jwt/v5)
- **Google APIs**: Google Play Android Developer API
- **Logging**: Uber Zap
- **Configuration**: Environment-based configuration

## Build and Development Commands

```bash
# Install dependencies
go mod download

# Build the project
go build ./...

# Run the application
go run ./...

# Format code
go fmt ./...

# Vet code for issues
go vet ./...

# Update dependencies
go mod tidy

# Run tests (when implemented)
go test ./...

# Run specific test
go test -run TestFunctionName ./path/to/package
```

## Architecture

The codebase follows a **domain-driven design** with clean separation of concerns:

```
internal/
├── config/          # Environment-based configuration management
├── models/          # Database models and business entities
└── services/        # Core business logic and external integrations
```

### Core Components

1. **Configuration Management** (`internal/config/config.go`)
   - Environment-based configuration with defaults
   - Supports database, Redis, Google Play, JWT, and server settings
   - All settings configurable via environment variables

2. **Data Models** (`internal/models/models.go`)
   - **User**: User entity with UUID-based identification
   - **Product**: Google Play product definitions
   - **Purchase**: One-time purchase tracking with states (PENDING, PURCHASED, CANCELLED, REFUNDED, EXPIRED)
   - **Subscription**: Subscription management with states (ACTIVE, CANCELLED, EXPIRED, ON_HOLD, PAUSED, PENDING, IN_GRACE_PERIOD)
   - **WebhookEvent**: Google Play webhook event processing

3. **Google Play Service** (`internal/services/google_play_service.go`)
   - Purchase and subscription verification
   - Purchase acknowledgment and consumption
   - Webhook signature validation
   - Subscription status determination

### Key Features

- **Purchase Verification**: Validates one-time purchases with Google Play
- **Subscription Management**: Handles subscription lifecycle events
- **Webhook Processing**: Processes real-time billing events from Google Play
- **State Management**: Tracks purchase and subscription states with proper transitions
- **Error Handling**: Comprehensive logging and error management

## Environment Configuration

Required environment variables:

```bash
# Server
SERVER_PORT=8080
SERVER_MODE=release

# Database
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=your_password
DB_NAME=billing

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Google Play
GOOGLE_SERVICE_ACCOUNT_FILE=service-account.json
GOOGLE_PACKAGE_NAME=com.example.app
GOOGLE_WEBHOOK_SECRET=your_webhook_secret

# JWT
JWT_SECRET=your-secret-key
```

## Development Notes

- The service requires a Google Play service account with Android Developer API access
- Webhook signature verification is implemented but uses placeholder logic
- Database migrations are not yet implemented
- No API routes or HTTP handlers are currently implemented
- Testing framework needs to be established

## Next Steps for Development

1. Implement HTTP handlers and API routes using Gin framework
2. Set up database migrations for the models
3. Implement proper webhook signature verification
4. Add comprehensive test coverage
5. Create Docker configuration for containerization
6. Add API documentation and validation