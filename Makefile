
VERSION := $(shell cat VERSION)

proto: proto/video_daemon.proto
	buf generate

build:
	go build -o ./bin/go-video-daemon \
		-ldflags "-X main.Version=$(VERSION)" \
		cmd/server/main.go

docker:
	docker build -t video-player-server .

test:
	@echo "==> Running unit tests"
	go test ./... -v

test-coverage:
	@echo "==> Running tests with coverage"
	go test ./... -coverprofile=coverage.out

.PHONY: proto build docker test test-coverage