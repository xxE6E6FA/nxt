BINARY = nxt

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: build the binary
.PHONY: build
build:
	go build -o ${BINARY} .

## run: build and run
.PHONY: run
run: build
	./${BINARY}

## test: run all tests with race detector
.PHONY: test
test:
	go test -race ./...

## cover: run tests with coverage report
.PHONY: cover
cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "\nHTML report: coverage.html"
	go tool cover -html=coverage.out -o coverage.html

## lint: run golangci-lint
.PHONY: lint
lint:
	go tool golangci-lint run

## fmt: format code with golangci-lint formatters
.PHONY: fmt
fmt:
	go tool golangci-lint fmt

## vet: run go vet
.PHONY: vet
vet:
	go vet ./...

## vuln: check for known vulnerabilities
.PHONY: vuln
vuln:
	go tool govulncheck ./...

## audit: run all quality checks (vet, lint, test, vuln)
.PHONY: audit
audit: vet lint test vuln

## tidy: tidy and verify module dependencies
.PHONY: tidy
tidy:
	go mod tidy
	go mod verify

## clean: remove build artifacts
.PHONY: clean
clean:
	rm -f ${BINARY} coverage.out coverage.html
