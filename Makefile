# Makefile - Unified build system for Cheapskate Finance Tracker
# Supports: Linux, Docker, Android (ARM64)
.PHONY: all build run generate tools clean hooks-cli setup-hooks test vendor \
        build-android build-android-apk docker dev

all: build

# ─── Tools ──────────────────────────────────────────────────────────────
tools:
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/air-verse/air@latest

# ─── Code Generation ────────────────────────────────────────────────────
generate:
	sqlc generate
	templ generate

# ─── Build: Linux (default) ─────────────────────────────────────────────
build: generate
	CGO_ENABLED=0 go build -o bin/server ./server
	@echo "Built: bin/server (linux/$(shell go env GOARCH))"

# ─── Build: Android ARM64 ───────────────────────────────────────────────
# Produces a static Linux ARM64 binary that runs on Android.
# Android is Linux-based, so GOOS=linux works for Android ARM64 devices.
build-android: generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/server-android-arm64 ./server
	@echo "Built: bin/server-android-arm64 (android/arm64)"

# Build Android APK (requires Android SDK + Gradle in android/)
# First builds the server binary, copies it to Android assets, then builds the APK.
build-android-apk: build-android
	mkdir -p android/app/src/main/assets
	cp bin/server-android-arm64 android/app/src/main/assets/server-arm64
	cd android && ./gradlew assembleDebug
	@echo "APK built: android/app/build/outputs/apk/debug/app-debug.apk"

# ─── Build: Docker ──────────────────────────────────────────────────────
docker:
	docker build -t cheapskate .

# ─── Vendor: Download frontend dependencies ─────────────────────────────
vendor:
	mkdir -p client/assets/vendor
	curl -sL "https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js" \
		-o client/assets/vendor/htmx.min.js
	@echo "Vendor assets downloaded to client/assets/vendor/"

# ─── Testing ────────────────────────────────────────────────────────────
test:
	CGO_ENABLED=0 go test ./... -v

# ─── Run ────────────────────────────────────────────────────────────────
run: generate
	go run ./server

dev:
	export PATH=$(PATH):$(HOME)/go/bin && air

# ─── Git Hooks ──────────────────────────────────────────────────────────
hooks-cli:
	go build -o bin/hooks-cli ./scripts/hooks-cli

setup-hooks: hooks-cli
	./bin/hooks-cli setup-hooks

# ─── Clean ──────────────────────────────────────────────────────────────
clean:
	rm -rf bin
	rm -rf android/app/build
