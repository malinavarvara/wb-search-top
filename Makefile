BINARY_NAME=searcher_top_bin
MAIN_PATH=./searcher_top/main.go

.PHONY: all build run fmt vet test lint clean up down docker-logs docker-kafka-logs kafka-create-topic kafka-list-topics kafka-produce

all: fmt vet lint build

build:
	CGO_ENABLED=0 go build -o bin/$(BINARY_NAME) $(MAIN_PATH)

run:
	go run $(MAIN_PATH)

test:
	go test -v -race ./...

lint:
	golangci-lint run -E gocritic -v ./...

clean:
	rm -rf bin/

up:
	docker compose up --build -d

down:
	docker compose down

docker-logs:
	docker compose logs -f app

docker-kafka-logs:
	docker compose logs -f kafka

kafka-create-topic:
	docker compose exec kafka kafka-topics \
		--bootstrap-server localhost:9092 \
		--create --if-not-exists \
		--topic search-logs \
		--partitions 3 \
		--replication-factor 1

kafka-list-topics:
	docker compose exec kafka kafka-topics \
		--bootstrap-server localhost:9092 \
		--list

kafka-produce:
	@echo '{"query":"кроссовки найк","user_id":"u-test-1","timestamp":"$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"}' | \
		docker compose exec -T kafka kafka-console-producer \
		--bootstrap-server localhost:9092 \
		--topic search-logs

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"