default: build

.PHONY: build test testacc lint fmt

build:
	go build ./...

test:
	go test ./... -timeout=5m

# Acceptance tests hit a real Sazabi organization. Requires TF_ACC=1,
# SAZABI_API_KEY, and SAZABI_ORGANIZATION_ID for a sandbox org.
testacc:
	TF_ACC=1 go test ./internal/provider/ -v -timeout=30m

lint:
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed on:" && gofmt -l . && exit 1)
	go vet ./...

fmt:
	gofmt -w .
