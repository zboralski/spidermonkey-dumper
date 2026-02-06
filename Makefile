OUT := sm33dis
PKG := github.com/zboralski/spidermonkey-dumper

.PHONY: all build test vet clean run decompile

all: build

build:
	go build -o $(OUT) ./cmd/sm33dis

test:
	go test -v ./...

vet:
	go vet ./...

clean:
	rm -f $(OUT)

run: build
	@if [ -z "$(FILE)" ]; then echo "usage: make run FILE=path/to/script.jsc"; exit 1; fi
	./$(OUT) $(FILE)

decompile: build
	@if [ -z "$(FILE)" ]; then echo "usage: make decompile FILE=path/to/script.jsc"; exit 1; fi
	./$(OUT) --decompile $(FILE)
