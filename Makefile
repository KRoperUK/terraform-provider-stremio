PLUGIN_DIR := $(HOME)/.terraform.d/plugins/registry.terraform.io/kroperuk/stremio/0.0.1/linux_amd64
BINARY     := terraform-provider-stremio

.PHONY: build install dev fmt vet

## Build the provider binary
build:
	go build -o $(BINARY) .

## Build and install into the local plugin cache for dev_overrides
install: build
	mkdir -p $(PLUGIN_DIR)
	cp $(BINARY) $(PLUGIN_DIR)/$(BINARY)
	@echo "Installed to $(PLUGIN_DIR)"

## Run go fmt
fmt:
	go fmt ./...

## Run go vet
vet:
	go vet ./...
