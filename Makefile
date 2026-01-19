.PHONY: all build test lint clean install tidy

BINARY_NAME=apprun-dedicated-provisioner
BINARY_PATH=bin/$(BINARY_NAME)
CMD_PATH=./cmd/$(BINARY_NAME)

all: build

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
