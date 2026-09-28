BIN := notify-agent
PKG := ./cmd/notify-agent
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# 순수 Go로 유지한다(SQLite도 modernc 순수 Go 판).
export CGO_ENABLED = 0

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet check install uninstall

build:
	go build -ldflags="$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./... -timeout 120s

vet:
	go vet ./...

check: vet test

# 맥에 설치하고 로그인할 때마다 켜지게 한다(LaunchAgent). scripts/install.sh 참고.
install: build
	./scripts/install.sh

uninstall:
	./scripts/install.sh --uninstall
