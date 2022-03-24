NOW=`date '+%Y.%m.%d %H:%M:%S'`
OS=`uname -n -m`
AFTER_COMMIT=`git rev-parse HEAD`
VERSION=0.7.0
BIN_FOLDER=build
BIN_NAME=ktest

build: clear
	go build -ldflags "-X 'main.BuildVersion=$(VERSION)' -X 'main.BuildTime=$(NOW)' -X 'main.BuildOSUname=$(OS)' -X 'main.BuildCommit=$(AFTER_COMMIT)'" -o $(BIN_FOLDER)/$(BIN_NAME) ./cmd/ktest

release: clear
	go run ./_script/release.go -build-version="$(VERSION)" -build-time="$(NOW)" -build-uname="$(OS)" -build-commit="$(AFTER_COMMIT)"

clear:
	if [ -d $(BIN_FOLDER) ]; then rm -r $(BIN_FOLDER); fi

.PHONY: build release
