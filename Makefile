# Local Server
local-dev:
	@go run ./cmd/gocore/main.go

# Docker
docker-dev:
	@docker-compose -f docker-compose-dev.yaml up -d