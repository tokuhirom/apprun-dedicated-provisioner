.PHONY: all build test lint clean install tidy generate

BINARY_NAME=apprun-dedicated-provisioner
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=./cmd/$(BINARY_NAME)

all: build

generate:
	go run github.com/ogen-go/ogen/cmd/ogen@latest --target api --clean openapi.json

build:
	go build -o $(BINARY_PATH) $(CMD_PATH)

test:
	go test ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

install:
	go install $(CMD_PATH)

tidy:
	go mod tidy
