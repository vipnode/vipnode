# Protips:
# - RUN can be overridden for debugging, like:
#   $ RUN="dlv debug --" make -e fakepool

PKG := $(shell go list | head -n1)
VERSION := $(shell git describe --tags --dirty --always 2> /dev/null || echo "dev")
LDFLAGS = "-X main.Version=$(VERSION)"
SOURCES = $(shell find . -type f -name '*.go')

BINARY = $(notdir $(PWD))
RUN = ./$(BINARY)

# Configs
FAKEBIND = 127.0.0.1:8080
FAKEPEERS = 0

all: $(BINARY)

$(BINARY): $(SOURCES)
	GO111MODULE=on go build -ldflags $(LDFLAGS) -o "$@"

deps:
	GO111MODULE=on go get -d

build: $(BINARY)

clean:
	rm $(BINARY)

run: $(BINARY)
	$(RUN) --help

debug: $(BINARY)
	$(RUN) -vv

test:
	go test -vet "all" -timeout 5s -race ./...

fakepool: $(BINARY)
	$(RUN) -vv pool --bind "$(FAKEBIND)" --store="memory" --allow-origin "http://localhost:3000"

fakehost: $(BINARY)
	$(RUN) -vv host --pool "ws://$(FAKEBIND)" --enode="enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@[::]:30303?discport=0" --rpc "fakenode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1?fakepeers=$(FAKEPEERS)"

fakehostpool: $(BINARY)
	$(RUN) -vv host --pool ":memory:" --enode="enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@[::]:30303?discport=0" --rpc "fakenode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1?fakepeers=$(FAKEPEERS)"

fakeclient: $(BINARY)
	$(RUN) -vv client "http://$(FAKEBIND)" --nodekey=./nodekey --rpc "fakenode://85fbed4332ed4329ca2283f26606618815ae83a870c523bb99b0b2e9dfe5af3b4699c2830ecdeb67519d62362db44aa5a8cafee523e3ab8c76aeef1016f424a4?fakepeers=$(FAKEPEERS)"

release:
	GOOS=linux GOARCH=amd64 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
	GOOS=linux GOARCH=386 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
	GOOS=linux GOARCH=arm GOARM=6 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
	GOOS=darwin GOARCH=amd64 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
	GOOS=windows GOARCH=386 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
	# We use xgo to cross-compile and it does not support freebsd unfortunately: https://github.com/karalabe/xgo/issues/91
	#GOOS=freebsd GOARCH=amd64 LDFLAGS=$(LDFLAGS) ./build_release "$(PKG)" README.md LICENSE
