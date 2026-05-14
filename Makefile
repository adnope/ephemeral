.PHONY: build dev run lint clean docker docker-run

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/leandrop ./cmd/leandrop

dev:
	go run ./cmd/leandrop

run: build
	./bin/leandrop

lint:
	golangci-lint run ./...

clean:
	rm -rf ./bin ./data

docker:
	docker build -t leandrop:latest .

docker-run:
	docker run -d \
		--name leandrop \
		-p 8080:8080 \
		-v leandrop-data:/app/data \
		--restart unless-stopped \
		leandrop:latest
