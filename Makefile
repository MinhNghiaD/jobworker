-include .env

PROJECTNAME := $(shell basename "$(PWD)")

# Go related variables.
SOURCEDIR := "./cmd"
BIN := "./bin"
SOURCES := $(shell find $(SOURCEDIR) ! -name "*_test.go" -name '*.go')
SOURCES_TST := $(shell find $(SOURCEDIR) -name '*.go')
PROTODIR := "./api/worker/proto"

.PHONY: update-vendor
update-vendor:
	# update modules in root directory
	go mod tidy
	go mod vendor

.PHONY: proto
worker_service.pb.go: 
	protoc --go_out=. --go_opt=paths=source_relative  \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative  \
        $(PROTODIR)/worker_service.proto

proto: worker_service.pb.go

.PHONY: build
build:
	@-$(MAKE) clean
	@-$(MAKE) update-vendor
	@echo "  >  Building binary..."
	@-$(MAKE) proto
	go build -o $(BIN)/ ./...

.PHONY: clean
clean:
	@-rm -r $(BIN) 2> /dev/null
	@-$(MAKE) go-clean

go-clean:
	@echo "  >  Cleaning build cache"
	@go clean
