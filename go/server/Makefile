MAKEFLAGS += --no-builtin-rules
.SUFFIXES:

APP          := networkqualityd
GIT_VERSION  := $(shell git describe --always --long)
PKG          := github.com/network-quality/server/go/server
LDFLAGS      := -ldflags "-w -X $(PKG).GitVersion=$(GIT_VERSION)"
GO           ?= go

COMMON_GO_FILES := *.go

CMD_SOURCES     := $(shell find cmd -name main.go)
DEV_TARGETS     := $(patsubst cmd/%/main.go,%,$(CMD_SOURCES))

all: networkqualityd

test: $(APP)
	$(GO) test -cover ./...

vet:
	$(GO) vet ./...

test-race: $(APP)
	$(GO) test -race -cover ./...

lint:
	golangci-lint run

clean:
	@rm -f $(DEV_TARGETS)

%: CWD=$(PWD)
%: cmd/%/*.go $(COMMON_GO_FILES)
	cd cmd/$@ && $(GO) build -o $(CWD)/$@ $(LDFLAGS) .

.PHONY: all test vet test-race lint clean
