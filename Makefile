GO ?= go
BIN_DIR := bin
BIN := $(BIN_DIR)/grafana2signoz
CLI := ./cmd/grafana2signoz

.PHONY: help build install test fmt vet cover clean convert compare

help:
	@echo "Targets: build, install, test, fmt, vet, cover, clean, convert, compare"
	@echo "convert: make convert IN=<grafana.json|dir> OUT=<out.json|dir> [RULES=<file>] [DRYRUN=1]"
	@echo "compare: make compare GRAFANA=<grafana.json> SIGNOZ=<signoz.json> [RULES=<file>]"

$(BIN):
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CLI)

build: $(BIN)

install:
	$(GO) install ./cmd/grafana2signoz

test:
	$(GO) test ./...

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	@$(GO) tool cover -func=coverage.out | tail -n 1 || true

clean:
	rm -rf $(BIN_DIR) coverage.out

# Usage: make convert IN=grafana-dasboards OUT=converted-signoz
convert: $(BIN)
	@if [ -z "$(IN)" ]; then echo "IN not set"; exit 1; fi
	@if [ -n "$(DRYRUN)" ]; then \
		$(BIN) convert --input $(IN) --dry-run $(if $(RULES),--rules $(RULES),) ; \
	else \
		if [ -z "$(OUT)" ]; then echo "OUT not set"; exit 1; fi; \
		$(BIN) convert --input $(IN) --output $(OUT) $(if $(RULES),--rules $(RULES),) ; \
	fi

# Usage: make compare GRAFANA=grafana.json SIGNOZ=signoz.json
compare: $(BIN)
	@if [ -z "$(GRAFANA)" ]; then echo "GRAFANA not set"; exit 1; fi
	@if [ -z "$(SIGNOZ)" ]; then echo "SIGNOZ not set"; exit 1; fi
	$(BIN) compare --grafana $(GRAFANA) --signoz $(SIGNOZ) $(if $(RULES),--rules $(RULES),)
