BINARY := cctask
GOBIN := /opt/homebrew/bin/go

.PHONY: build test install clean vet

build:
	$(GOBIN) build -o $(BINARY) .

test:
	$(GOBIN) test ./internal/... -v

install: build
	cp $(BINARY) /opt/homebrew/bin/$(BINARY)

vet:
	$(GOBIN) vet ./...

clean:
	rm -f $(BINARY)
