NOW=`date '+%Y.%m.%d %H:%M:%S'`
OS=`uname -n -m`
AFTER_COMMIT=`git rev-parse HEAD`
VERSION=0.6.0

release:
	go run ./_script/release.go -build-version="$(VERSION)" -build-time="$(NOW)" -build-uname="$(OS)" -build-commit="$(AFTER_COMMIT)"

.PHONY: release
