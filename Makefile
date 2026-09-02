.PHONY: dev build test docker-build docker-up clean

# Development
dev-backend:
	cd backend && go run .

dev-frontend:
	cd frontend && npm run dev

dev: dev-backend dev-frontend

# Build
build-backend:
	cd backend && go build -o ../bin/idmonitor-server .
	cd backend && go build -o ../bin/idmonitor-worker ./worker.go

build-frontend:
	cd frontend && npm run build

build: build-backend build-frontend

# Test
test-backend:
	cd backend && go test ./...

test-frontend:
	cd frontend && npm run test

test: test-backend test-frontend

# Docker
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

# Database
db-migrate:
	cd backend && go run .

# Clean
clean:
	rm -rf bin/ frontend/dist/ node_modules/
