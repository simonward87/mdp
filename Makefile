## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N]' && read ans && [ $${ans:-N} = y ]

## start: run markdown preview with example.md
.PHONY: start
start:
	@go run ./cmd/mdp/ ./example.md

## build: compile production binaries
.PHONY: build
build:
	@echo 'Minifying templates...'
	@mv ./web/template/default.tmpl temp.default.tmpl
	@minify -q -o ./web/template/default.tmpl temp.default.tmpl
	@echo 'Building binaries...'
	@go build -ldflags='-s -w' -o=./bin/darwin_arm64/mdp ./cmd/mdp/
	@GOOS=linux GOARCH=arm64 go build -ldflags='-s -w' -o=./bin/linux_arm64/mdp ./cmd/mdp/
	@echo 'Binaries built successfully'
	@echo 'Restoring human-readable templates...'
	@rm ./web/template/default.tmpl
	@mv ./temp.default.tmpl ./web/template/default.tmpl
	@echo 'Done'
