.PHONY: build run test clean docker-build docker-run tf-init tf-plan tf-apply lint fmt mod-tidy help

# Go commands
build:
	go build -o bin/server ./cmd/server

run:
	go run ./cmd/server

test:
	go test -v ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

clean:
	rm -rf bin/
	rm -f data.db
	rm -f coverage.out coverage.html

# Docker commands
docker-build:
	docker build -t pavlok-llm-manager .

docker-run:
	docker run -p 8080:8080 --env-file .env pavlok-llm-manager

# Local development with SQLite
dev:
	DB_TYPE=sqlite go run ./cmd/server

# Terraform commands
tf-init:
	cd terraform && terraform init

tf-plan:
	cd terraform && terraform plan -var-file="terraform.tfvars"

tf-apply:
	cd terraform && terraform apply -var-file="terraform.tfvars"

tf-destroy:
	cd terraform && terraform destroy -var-file="terraform.tfvars"

tf-fmt:
	cd terraform && terraform fmt

tf-validate:
	cd terraform && terraform validate

# Utility
lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

mod-tidy:
	go mod tidy

# CI commands (for local testing before push)
ci-test: test lint
	@echo "All CI checks passed!"

ci-build: build docker-build
	@echo "Build completed!"

# Help
help:
	@echo "Available commands:"
	@echo ""
	@echo "Development:"
	@echo "  build         - Build the application"
	@echo "  run           - Run the application"
	@echo "  dev           - Run in development mode (SQLite)"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  clean         - Clean build artifacts"
	@echo ""
	@echo "Docker:"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"
	@echo ""
	@echo "Terraform:"
	@echo "  tf-init       - Initialize Terraform"
	@echo "  tf-plan       - Plan Terraform changes"
	@echo "  tf-apply      - Apply Terraform changes"
	@echo "  tf-destroy    - Destroy Terraform resources"
	@echo "  tf-fmt        - Format Terraform files"
	@echo "  tf-validate   - Validate Terraform configuration"
	@echo ""
	@echo "Code Quality:"
	@echo "  lint          - Run linter"
	@echo "  fmt           - Format Go code"
	@echo "  mod-tidy      - Tidy Go modules"
	@echo ""
	@echo "CI (local testing):"
	@echo "  ci-test       - Run all CI test checks locally"
	@echo "  ci-build      - Run all build checks locally"
	@echo ""
	@echo "Note: Deployment is handled via GitHub Actions."
	@echo "Push a tag (e.g., git tag v1.0.0 && git push origin v1.0.0) to trigger deployment."
