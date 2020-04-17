GO_DEPS := sqlc goconvey golangci-lint
SQLC_VERSION := v1.2.0
GOCONVEY_VERSION := v1.6.4
GOLANGCI_LINT_VERSION := v1.24.0

GOBIN := $(shell go env GOPATH)/bin

# Get the name of the tag associated with the current commit if there is one
# Otherwise, get the short SHA
CURRENT_REVISION := $(shell git describe --exact-match HEAD 2>/dev/null || git rev-parse --short HEAD)
BUILD_DATE := $(shell date -u "+%FT%T UTC")

.DEFAULT: all

.PHONY: all
all: check-deps build lint test

$(GO_DEPS):
	@which $@ > /dev/null || (echo "$@ not installed"; exit 1)

.PHONY: check-deps
check-deps: $(GO_DEPS)

.PHONY: install-sqlc
install-sqlc:
	GO111MODULE=on go get -d github.com/kyleconroy/sqlc/cmd/sqlc@$(SQLC_VERSION)
	go install github.com/kyleconroy/sqlc/cmd/sqlc

.PHONY: install-goconvey
install-goconvey:
	GO111MODULE=on go get -d github.com/smartystreets/goconvey@$(GOCONVEY_VERSION)
	go install github.com/smartystreets/goconvey

.PHONY: install-golangci-lint
install-golangci-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN) $(GOLANGCI_LINT_VERSION)

.PHONY: install-deps
install-deps: install-sqlc install-goconvey install-golangci-lint

.PHONY: docker
docker:
	docker build -t mihaitodor/ferrum .

.PHONY: generate
generate:
	$(GOBIN)/sqlc generate

.PHONY: build
build:
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static" -X "github.com/mihaitodor/ferrum/config.version=$(CURRENT_REVISION)" -X "github.com/mihaitodor/ferrum/config.buildDate=$(BUILD_DATE)"' ./cmd/ferrum

.PHONY: lint
lint: generate
	@if [ ! -z "$$( git status --porcelain db )" ]; then \
		echo "Please run 'sqlc generate' in the current branch"; \
		exit 1; \
	fi

	go mod tidy
	@if [ ! -z "$$( git status --porcelain go.mod go.sum )" ]; then \
		echo "Please run 'go mod tidy' in the current branch"; \
		exit 1; \
	fi

	$(GOBIN)/golangci-lint run

.PHONY: test
test:
	go test -v ./...

.PHONY: test-integration
test-integration:
	./integration_test.sh

.PHONY: up
up:
	docker-compose up

.PHONY: down
down:
	docker-compose down

.PHONY: clean
clean:
	rm -rf ferrum