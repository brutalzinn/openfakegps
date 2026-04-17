.PHONY: build-android build-android-release install-android install-android-all launch-android build-backend run-backend docker-up docker-down clean

# Android
ANDROID_DIR := android
APK_DEBUG := $(ANDROID_DIR)/app/build/outputs/apk/debug/app-debug.apk
APK_RELEASE := $(ANDROID_DIR)/app/build/outputs/apk/release/app-release.apk
APK := $(APK_DEBUG)
PACKAGE := com.openfakegps.agent
ACTIVITY := $(PACKAGE).MainActivity
JAVA_HOME ?= /Applications/Android Studio.app/Contents/jbr/Contents/Home

build-android:
	cd $(ANDROID_DIR) && JAVA_HOME="$(JAVA_HOME)" ./gradlew assembleDebug

build-android-release:
	cd $(ANDROID_DIR) && JAVA_HOME="$(JAVA_HOME)" ./gradlew assembleRelease

## Install on a specific device: make install-android DEVICE=<serial>
## Use MODE=release for release build: make install-android MODE=release
install-android: $(if $(filter release,$(MODE)),build-android-release,build-android)
ifndef DEVICE
	adb install -r $(if $(filter release,$(MODE)),$(APK_RELEASE),$(APK_DEBUG))
else
	adb -s $(DEVICE) install -r $(if $(filter release,$(MODE)),$(APK_RELEASE),$(APK_DEBUG))
endif

## Install on ALL connected devices
install-android-all: $(if $(filter release,$(MODE)),build-android-release,build-android)
	@adb devices | grep -w device | grep -v List | awk '{print $$1}' | while read serial; do \
		echo "Installing on $$serial..."; \
		adb -s $$serial install -r $(if $(filter release,$(MODE)),$(APK_RELEASE),$(APK_DEBUG)); \
	done

## Launch the app on a specific device (or default)
launch-android:
ifndef DEVICE
	adb shell am start -n $(PACKAGE)/$(ACTIVITY)
else
	adb -s $(DEVICE) shell am start -n $(PACKAGE)/$(ACTIVITY)
endif

## Launch on ALL connected devices
launch-android-all:
	@adb devices | grep -w device | grep -v List | awk '{print $$1}' | while read serial; do \
		echo "Launching on $$serial..."; \
		adb -s $$serial shell am start -n $(PACKAGE)/$(ACTIVITY); \
	done

# Backend
build-backend:
	cd backend && go build ./...

run-backend:
	cd backend && go run ./cmd/server

test-backend:
	cd backend && go test ./...

# Docker
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

# Cleanup
clean:
	cd $(ANDROID_DIR) && JAVA_HOME="$(JAVA_HOME)" ./gradlew clean
	cd backend && go clean ./...

# Help
help:
	@echo "Android:"
	@echo "  make build-android                       Build debug APK"
	@echo "  make build-android-release                Build release APK"
	@echo "  make install-android                      Install debug on default device"
	@echo "  make install-android MODE=release          Install release on default device"
	@echo "  make install-android DEVICE=xyz            Install on specific device"
	@echo "  make install-android-all                   Install on ALL connected devices"
	@echo "  make install-android-all MODE=release      Install release on ALL devices"
	@echo "  make launch-android                        Launch app on default device"
	@echo "  make launch-android-all                    Launch app on ALL connected devices"
	@echo ""
	@echo "Backend:"
	@echo "  make build-backend               Build Go backend"
	@echo "  make run-backend                 Run backend server"
	@echo "  make test-backend                Run backend tests"
	@echo ""
	@echo "Docker:"
	@echo "  make docker-up                   Start backend with Docker"
	@echo "  make docker-down                 Stop Docker services"
	@echo ""
	@echo "Other:"
	@echo "  make clean                       Clean build artifacts"
