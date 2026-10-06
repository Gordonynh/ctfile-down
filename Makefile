APP       := ctfile-down
VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo 0.2.0)
LDFLAGS   := -s -w -X main.version=$(VERSION)
DIST      := dist
GOOS_GOARCH := \
	windows/amd64 \
	windows/arm64 \
	linux/amd64 \
	linux/arm64 \
	linux/386 \
	linux/arm/7 \
	darwin/amd64 \
	darwin/arm64

.PHONY: all build build-all app dmg release-all clean vet test

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP) .

vet:
	go vet ./...

test:
	go test ./...

# macOS .app（仅 darwin 主机）
app:
	VERSION=$(VERSION) bash macos/build-app.sh

# macOS DMG（仅 darwin 主机）
dmg: app
	VERSION=$(VERSION) bash macos/make-dmg.sh

# 全平台二进制 + macOS .app/DMG
release-all: build-all
	@if [ "$$(uname)" = "Darwin" ]; then $(MAKE) dmg; else echo "(跳过 .app/DMG：非 macOS 主机)"; fi

build-all: clean
	@mkdir -p $(DIST)
	@for target in $(GOOS_GOARCH); do \
		goos=$${target%%/*}; rest=$${target#*/}; \
		goarch=$${rest%%/*}; goarm=$${rest#*/}; \
		name=$(APP); ext=""; \
		[ "$$goos" = "windows" ] && ext=".exe"; \
		out="$(DIST)/$(APP)_$${goos}_$${goarch}"; \
		[ "$$goarm" != "$$goarch" ] && out="$${out}v$${goarm}"; \
		out="$${out}$${ext}"; \
		echo ">> building $$goos/$$goarch (arm=$$goarm) -> $$out"; \
		GOOS=$$goos GOARCH=$$goarch GOARM=$$goarm CGO_ENABLED=0 \
			go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" . || exit 1; \
	done
	@echo ""
	@echo "构建完成，产物位于 $(DIST)/："
	@ls -lh $(DIST)

clean:
	rm -rf $(DIST) $(APP) .build
