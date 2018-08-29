BINARY = $(notdir $(PWD))
VERSION := $(shell git describe --tags --dirty --always 2> /dev/null || echo "dev")
SOURCES = $(wildcard *.go **/*.go)
PKG := $(shell go list | head -n1)

all: $(BINARY)

$(BINARY): $(SOURCES)
	go build -ldflags "-X main.Version=$(VERSION)" -o "$@"

deps:
	go get ./...

build: $(BINARY)

clean:
	rm $(BINARY)

run: $(BINARY)
	./$(BINARY) --help

debug: $(BINARY)
	./$(BINARY) -vv

test:
	go test -vet "" -race ./...

fakepool: $(BINARY)
	./$(BINARY) -vv pool --bind "127.0.0.1:8080" --store="memory"

fakehost: $(BINARY)
	./$(BINARY) -vv host --pool "ws://127.0.0.1:8080" --enode="enode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1@[::]:30303?discport=0" --rpc "fakenode://f21f0692b06019ae3f40d78d8b309487fc75f75b76df71d76196c3514272adf30aca4b2451181eb22208757cd4363923e17723d2f2ddf7b0175ecb87dada7ca1"

fakeclient: $(BINARY)
	./$(BINARY) -vv client "ws://127.0.0.1:8080" --rpc "fakenode://4ec357a8409303a0fcced83ed2751ff14ed17c3764c617bf98bbef3f048bbd3e03d6732d234ab7c1c5426a22efa6934997c1f8979482d2360ffeef9fc7cc2c9"
