MODULE := github.com/laddawan/backend-test

.PHONY: run build test test-integration cover proto docker-up docker-down fmt vet

run:            ## Run the server locally (needs MongoDB and JWT_SECRET)
	go run .

build:          ## Build the binary into ./bin
	go build -o bin/server .

test:           ## Run unit tests
	go test ./...

test-integration: ## Run MongoDB integration tests (needs MONGO_URI)
	go test -tags=integration ./repository/mongodb/...

cover:          ## Unit tests with coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

proto:          ## Regenerate gRPC code (needs protoc, protoc-gen-go, protoc-gen-go-grpc)
	protoc -I proto \
		--go_out=. --go_opt=module=$(MODULE) \
		--go-grpc_out=. --go-grpc_opt=module=$(MODULE) \
		proto/user/v1/user.proto

docker-up:      ## Start API + MongoDB
	docker compose up --build -d

docker-down:    ## Stop containers
	docker compose down

fmt:
	gofmt -w .

vet:
	go vet ./...
