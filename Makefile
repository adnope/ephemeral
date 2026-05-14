.PHONY: build dev run format lint fix clean clean-data docker docker-run \
	format-go lint-go format-web lint-web fix-web install-web

APP_NAME := ephemeral
WEB_DIR := web

GO_DIRS := ./cmd/... ./internal/...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/$(APP_NAME) ./cmd/$(APP_NAME)

dev:
	air

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

docker:
	docker build -t $(APP_NAME):latest .

docker-run:
	docker run -d \
		--name $(APP_NAME) \
		-p 8080:8080 \
		-v $(APP_NAME)-data:/app/data \
		--restart unless-stopped \
		$(APP_NAME):latest
