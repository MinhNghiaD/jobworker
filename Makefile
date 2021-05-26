-include .env

PROJECTNAME := $(shell basename "$(PWD)")

# Go related variables.
SOURCEDIR := "./cmd"
BIN := "./bin"
SCRIPT := "./scripts"
SOURCES := $(shell find $(SOURCEDIR) ! -name "*_test.go" -name '*.go')
PROTODIR := "./api/worker/proto"

.PHONY: update-vendor
update-vendor:
	@echo "  > Update modules" 
	@go mod tidy

.PHONY: proto
worker_service.pb.go: 
	@protoc --go_out=. --go_opt=paths=source_relative  \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative  \
        $(PROTODIR)/worker_service.proto

proto: worker_service.pb.go

.PHONY: build
build:
	@-$(MAKE) clean
	@-$(MAKE) proto
	@-$(MAKE) update-vendor
	@-$(MAKE) cert
	@echo "  >  Building binary..."
	@go build -o $(BIN)/ ./...
	@-$(MAKE) token

.PHONY: test
test-log:	
	@go test -race ./pkg/log/... 

test-job:	
	@go test -race ./pkg/job/...

test-grpc:
	@go test -race ./pkg/service/...

test-auth:
	@-$(MAKE) cert
	@-$(MAKE) token
	@go test -race ./pkg/auth/...

test: test-log test-job test-grpc test-auth

.PHONY: clean
clean:
	@-rm -r $(BIN) 2> /dev/null
	@-$(MAKE) go-clean

go-clean:
	@echo "  >  Cleaning build cache"
	@go clean

.PHONY: lint
lint-go: GO_LINT_FLAGS ?=
lint-go:
	golangci-lint run -c .golangci.yaml $(GO_LINT_FLAGS)

lint: lint-go

.PHONY: cert
cert:
	@echo "  >  Generating certificates"
	@-bash $(SCRIPT)/cert_gen.sh

.PHONY: token
token:
	@echo "  >  Generating JWT tokens"
	@-bash $(SCRIPT)/jwt_gen.sh