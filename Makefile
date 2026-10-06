APP       := ctfile-down
VERSION   ?= $(shell git describe --tags --always 2>/dev/null || echo 0.1.0)
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

.PHONY: all build build-all clean vet test

all: build

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(APP) .

vet:
	go vet ./...

test:
	go test ./...

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
	rm -rf $(DIST) $(APP)
