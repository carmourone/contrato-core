.PHONY: run test tidy

run:
	go run ./cmd/contrato

test:
	go test ./...

tidy:
	go mod tidy
