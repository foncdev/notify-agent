BIN := notify-agent
PKG := ./cmd/notify-agent
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# 순수 Go로 유지한다(SQLite도 modernc 순수 Go 판).
export CGO_ENABLED = 0

LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test vet check install uninstall dist

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

# 릴리스 묶음을 만든다(terminal-agent·claudeAgent와 같은 이름 규칙 — install.sh가 이 이름을 쓴다):
#   dist/notify-agent_darwin_<arch>.tar.gz   안에 notify-agent, env.example, scripts/install.sh, README.md
#   dist/checksums.txt
# 맥 전용이다(알림 센터 DB를 읽는다). 서명·공증하면 새 판을 깔아도 전체 디스크 접근이 이어지기 쉽다:
#   DEVELOPER_ID="Developer ID Application: 이름 (TEAMID)" NOTARY_PROFILE=notary make dist
DIST := dist
PLATFORMS := darwin/arm64 darwin/amd64

dist:
	@rm -rf $(DIST) && mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; d=$(DIST)/$${os}_$${arch}; \
		echo "  $$os/$$arch"; \
		mkdir -p $$d/scripts; \
		GOOS=$$os GOARCH=$$arch go build -ldflags="$(LDFLAGS)" -o $$d/$(BIN) $(PKG) || exit 1; \
		if [ -n "$(DEVELOPER_ID)" ]; then \
			codesign --force --options runtime --timestamp --sign "$(DEVELOPER_ID)" $$d/$(BIN) || exit 1; \
			if [ -n "$(NOTARY_PROFILE)" ]; then \
				(cd $$d && zip -q notarize.zip $(BIN)) && \
				xcrun notarytool submit $$d/notarize.zip --keychain-profile "$(NOTARY_PROFILE)" --wait || exit 1; \
				rm -f $$d/notarize.zip; \
			fi; \
		fi; \
		cp README.md env.example $$d/ && cp scripts/install.sh $$d/scripts/; \
		tar -czf $(DIST)/$(BIN)_$${os}_$${arch}.tar.gz -C $$d $(BIN) env.example scripts/install.sh README.md || exit 1; \
	done
	@cd $(DIST) && shasum -a 256 *.tar.gz > checksums.txt
	@ls -lh $(DIST)/*.tar.gz $(DIST)/checksums.txt
