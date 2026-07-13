.PHONY: build build-web dev run test format lint fix clean clean-data docker docker-run \
	format-go lint-go format-web lint-web fix-web install-web

APP_NAME := ephemeral
WEB_DIR := web

GO_DIRS := ./cmd/... ./internal/... ./web

build: build-web
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/$(APP_NAME) ./cmd/$(APP_NAME)

build-web:
	cd $(WEB_DIR) && npm run build

dev:
	./web/node_modules/.bin/concurrently -k -n web,api "npm --prefix web run dev" "air"

test: build-web
	go test ./cmd/... ./internal/... ./web
	cd $(WEB_DIR) && npm test

run: build
	./bin/$(APP_NAME)

format: format-go format-web

format-go:
	go fmt $(GO_DIRS)

format-web:
	cd $(WEB_DIR) && npm run format

lint: lint-go lint-web

lint-go:
	go vet $(GO_DIRS)
	golangci-lint run $(GO_DIRS)

lint-web:
	cd $(WEB_DIR) && npm run lint

fix: format-go fix-web lint-go

fix-web:
	cd $(WEB_DIR) && npm run fix

install-web:
	cd $(WEB_DIR) && npm install

clean:
	rm -rf ./bin

clean-data:
	rm -rf ./data

docker-build:
	docker build -t adnope/$(APP_NAME):indev .

docker-up: docker-build
	docker compose down
	docker compose up -d
