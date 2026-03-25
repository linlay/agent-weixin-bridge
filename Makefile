VERSION := $(shell cat VERSION 2>/dev/null || echo "dev")
ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
COMPOSE_FILE ?= compose.yml

.PHONY: build run test docker-build docker-up docker-down release clean

build:
	mkdir -p bin
	go build -trimpath -ldflags="-s -w" -o bin/agent-weixin-bridge ./cmd/bridge

run:
	set -a; [ ! -f .env ] || . ./.env; set +a; go run ./cmd/bridge

test:
	go test ./...

docker-build:
	docker compose -f $(COMPOSE_FILE) build

docker-up:
	docker compose -f $(COMPOSE_FILE) up -d --build

docker-down:
	docker compose -f $(COMPOSE_FILE) down

release:
	VERSION=$(VERSION) ARCH=$(ARCH) RELEASE_BASE_IMAGE=$(RELEASE_BASE_IMAGE) RELEASE_BASE_IMAGE_LOCAL=$(RELEASE_BASE_IMAGE_LOCAL) bash scripts/release.sh

clean:
	rm -rf ./bin ./dist
