# sing-box features compiled into Harp. with_gvisor and with_wireguard are
# required; the rest enable QUIC based protocols, uTLS/REALITY and the Clash API.
TAGS ?= with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api
VERSION ?= dev
LDFLAGS ?= -s -w -buildid= -X github.com/gholamtaksir/Harp/cmd.Version=$(VERSION)
OUTPUT ?= harp$(if $(filter windows,$(shell go env GOOS)),.exe,)

.PHONY: build install test vet generate gui gui-package

build:
	CGO_ENABLED=0 go build -trimpath -tags "$(TAGS)" -ldflags "$(LDFLAGS)" -o "$(OUTPUT)" .

install:
	CGO_ENABLED=0 go install -trimpath -tags "$(TAGS)" -ldflags "$(LDFLAGS)" .

test:
	go test -tags "$(TAGS)" ./...

vet:
	go vet -tags "$(TAGS)" ./...

# Desktop app (separate Go module in gui/). Needs Node.js and the Wails CLI:
#   go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
# The binary is written to gui/bin/.
gui:
	cd gui && wails3 build EXTRA_TAGS="$(TAGS)" VERSION="$(VERSION)"

# Installer/package for the current OS (NSIS on Windows, .app on macOS,
# AppImage/deb/rpm on Linux).
gui-package:
	cd gui && wails3 package EXTRA_TAGS="$(TAGS)" VERSION="$(VERSION)"

generate:
	go generate
