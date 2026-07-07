.PHONY: run test fmt generate build docker-build docker-up docker-down

run:
	go run ./cmd/server

test:
	go test ./...

fmt:
	gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

generate:
	go generate ./...

build:
	go build -o /tmp/official-account-service ./cmd/server

docker-build:
	docker compose build

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down
