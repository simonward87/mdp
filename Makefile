## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N]' && read ans && [ $${ans:-N} = y ]

## start: serve website
.PHONY: start
start:
	@go run ./cmd/mdp/ -f ./example.md

## build: compile production binaries
.PHONY: build
build:
	@echo 'Building binaries...'
	@go build -ldflags='-s -w' -o=./bin/darwin_arm64/mdp ./cmd/mdp/
	@GOOS=linux GOARCH=arm64 go build -ldflags='-s -w' -o=./bin/linux_arm64/mdp ./cmd/mdp/
	@echo 'Binaries built successfully'
