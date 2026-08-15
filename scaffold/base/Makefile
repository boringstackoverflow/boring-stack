.PHONY: run build test

run:
	go run .

build:
	CGO_ENABLED=0 go build -trimpath -o app .

test:
	go test ./...
