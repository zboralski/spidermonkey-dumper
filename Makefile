OUT := smdis
PKG := github.com/zboralski/spidermonkey-dumper

.PHONY: all build test vet clean run decompile samples

all: build

build:
	go build -o $(OUT) ./cmd/smdis

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

samples: build
	@for f in samples/*.jsc; do \
		./$(OUT) "$$f" >/dev/null 2>&1; \
		./$(OUT) -callgraph "$$f" 2>&1; \
		./$(OUT) -controlflow "$$f" 2>&1; \
	done
