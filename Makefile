.PHONY: run build tidy clean setup pull-vision-model help

## run: start the development server
run:
	go run main.go

## build: compile to ./bin/app
build:
	@mkdir -p bin
	go build -o bin/app .
	@echo "✅ Built → bin/app"

## tidy: download dependencies and regenerate go.sum
tidy:
	go mod tidy

## clean: remove build artifacts
clean:
	rm -rf bin/

## setup: pull the LLaVA vision model
## This project has no Go dependencies beyond the standard library.
setup: pull-vision-model
	@echo ""
	@echo "✅ Ready. Run: make run → http://localhost:8080"

## pull-vision-model: pull llava:7b (~4.5 GB download)
pull-vision-model:
	@echo "Pulling llava:7b (this is a larger download, ~4.5 GB)..."
	ollama pull llava:7b

## help: list all available commands
help:
	@grep -E '^##' Makefile | sed 's/## //'