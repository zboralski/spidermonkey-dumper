# ===== Defaults (override on CLI if needed) =====
SM_SRC   ?= $(PWD)/mozilla-release/js/src
SM_BUILD ?= $(SM_SRC)/build
SM_DIST  ?= $(SM_BUILD)/dist
OUT      ?= dumper
SAMPLE   ?= samples/testdata/simple.jsc
SRC      := dumper.cpp utils.cpp logging.cpp ollama.cpp

# Build as arm64 by default for native compatibility with target JSC files
ARCH     ?= arm64

CXX      ?= clang++
CC       ?= clang
CXXFLAGS ?= -std=gnu++17 -O2 -g -arch $(ARCH) -Wformat -Wformat-security -Werror=format-security
CFLAGS   ?= -O2 -g -arch $(ARCH)
LDFLAGS  ?= -arch $(ARCH)


# Resolve the library from either the SpiderMonkey build tree or local ./dist/*
LIBJS := $(firstword \
  $(wildcard $(SM_DIST)/lib/libmozjs*.a) \
  $(wildcard $(SM_DIST)/lib/libmozjs*.dylib) \
  $(wildcard $(PWD)/dist/*/lib/libmozjs*.a) \
  $(wildcard $(PWD)/dist/*/lib/libmozjs*.dylib) \
  $(wildcard $(PWD)/dist/lib/libmozjs*.a) \
  $(wildcard $(PWD)/dist/lib/libmozjs*.dylib) \
)

#
# Derive DIST_ROOT robustly from the found lib path
LIBDIR := $(patsubst %/,%, $(dir $(LIBJS)))
DIST_ROOT := $(patsubst %/lib,%, $(LIBDIR))/

# --- libcurl (for Ollama / HTTP integrations) ---
# Use Homebrew curl with pkg-config
CURL_PREFIX    := /opt/homebrew/opt/curl
PKG_CONFIG_ENV := PKG_CONFIG_PATH="$(CURL_PREFIX)/lib/pkgconfig"
CURL_PKGCONFIG := $(shell $(PKG_CONFIG_ENV) pkg-config --exists libcurl && echo yes)
ifeq ($(CURL_PKGCONFIG),yes)
CURL_CFLAGS := $(shell $(PKG_CONFIG_ENV) pkg-config --cflags libcurl)
CURL_LIBS   := $(shell $(PKG_CONFIG_ENV) pkg-config --libs libcurl)
else
CURL_CFLAGS := -I$(CURL_PREFIX)/include
CURL_LIBS   := -L$(CURL_PREFIX)/lib -lcurl
endif

# --- nlohmann-json (header-only JSON library) ---
JSON_PREFIX := /opt/homebrew/opt/nlohmann-json
JSON_CFLAGS := -I$(JSON_PREFIX)/include

# Include roots: EXACT SOURCE TREE FIRST to avoid header layout drift
INCLUDES := \
  -I$(SM_SRC) \
  -I$(SM_DIST)/include \
  -I$(SM_DIST)/include/js \
  -I$(DIST_ROOT)include \
  -I$(DIST_ROOT)include/js \
  -I$(DIST_ROOT)include/mozjs-33 \
  -I$(DIST_ROOT)include/mozjs-33/js \
  -I$(DIST_ROOT)lib/include \
  -I$(DIST_ROOT)lib/include/js \
  -I/opt/homebrew/include/nspr \
  $(CURL_CFLAGS) \
  $(JSON_CFLAGS)

ifeq ($(strip $(LIBJS)),)
$(error Could not find libmozjs (searched $(SM_DIST)/lib and ./dist/*/lib) — build SpiderMonkey first)
endif

# Ensure packaged headers can find generated js-config.h
JSCONF_BUILD    := $(SM_DIST)/include/js-config.h
JSCONF_PACKAGED := $(DIST_ROOT)include/mozjs-33/js-config.h
# Fallback: look anywhere under DIST_ROOT include/lib trees
JSCONF_FALLBACK := $(firstword \
  $(wildcard $(DIST_ROOT)include/**/js-config.h) \
  $(wildcard $(DIST_ROOT)lib/include/**/js-config.h))

# Candidate locations for js-config.h (prefer real files in any build-* dist)
JS_CONF_CAND := \
  $(SM_SRC)/build-*/dist/include/js-config.h \
  $(SM_SRC)/build*/dist/include/js-config.h \
  $(SM_DIST)/include/js-config.h \
  $(DIST_ROOT)include/mozjs-33/js-config.h \
  $(DIST_ROOT)include/mozjs-33/js/js-config.h \
  $(DIST_ROOT)lib/include/mozjs-33/js-config.h

prepare-includes:
	@# Ensuring js-config.h is reachable from packaged headers
	@mkdir -p "$(DIST_ROOT)include/mozjs-33/js" "$(DIST_ROOT)lib/include/mozjs-33"
	@# Pick the first candidate that is a *regular file*
	@SRC=""; \
	for pat in $(JS_CONF_CAND); do \
	  for f in $$pat; do \
	    if [ -f "$$f" ]; then SRC="$$f"; break 2; fi; \
	  done; \
	done; \
	if [ -z "$$SRC" ]; then \
	  echo "[prep] js-config.h not found; searched:"; \
	  echo "       $(SM_SRC)/build-*/dist/include/js-config.h"; \
	  echo "       $(SM_SRC)/build*/dist/include/js-config.h"; \
	  echo "       $(SM_DIST)/include/js-config.h"; \
	  echo "       $(DIST_ROOT)include/mozjs-33/js-config.h"; \
	  echo "       $(DIST_ROOT)include/mozjs-33/js/js-config.h"; \
	  echo "       $(DIST_ROOT)lib/include/mozjs-33/js-config.h"; \
	  exit 1; \
	else \
	  : ; \
	fi; \
	rm -f "$(DIST_ROOT)include/mozjs-33/js-config.h" "$(DIST_ROOT)include/mozjs-33/js/js-config.h" "$(DIST_ROOT)lib/include/mozjs-33/js-config.h"; \
	cp -f "$$SRC" "$(DIST_ROOT)include/mozjs-33/js-config.h"; \
	ln -sf "../js-config.h" "$(DIST_ROOT)include/mozjs-33/js/js-config.h"; \
	ln -sf "../../include/mozjs-33/js-config.h" "$(DIST_ROOT)lib/include/mozjs-33/js-config.h"; \
	ls -l "$(DIST_ROOT)include/mozjs-33/js-config.h"; \
	ls -l "$(DIST_ROOT)include/mozjs-33/js/js-config.h"; \
	ls -l "$(DIST_ROOT)lib/include/mozjs-33/js-config.h"

# macOS system libs (usually fine as-is)
SYS_LIBS := -lz $(CURL_LIBS)

#
# ===== Targets =====

.PHONY: all clean print-sm prepare-includes test test-suite test-samples test-golden test-golden-update test-ollama test-basic test-inner test-resolve test-debug run print-curl

all: $(OUT)

$(OUT): $(SRC) | prepare-includes
	$(CXX) $(CXXFLAGS) $(INCLUDES) $(SRC) "$(LIBJS)" $(LDFLAGS) $(SYS_LIBS) -o $@

# Test build (same as main build now that Ollama is always enabled)
CXXFLAGS_TEST = $(CXXFLAGS)
TEST_BIN_DIR := tests/bin
OUT_TEST     := $(TEST_BIN_DIR)/$(OUT)

$(OUT_TEST): $(SRC) | prepare-includes
	@mkdir -p $(TEST_BIN_DIR)
	$(CXX) $(CXXFLAGS_TEST) $(INCLUDES) $(SRC) "$(LIBJS)" $(LDFLAGS) $(SYS_LIBS) -o $@

clean:
	rm -f $(OUT) $(OUT_TEST) *.o tests/actual/*.dis

print-sm:
	@echo "SM_SRC   = $(SM_SRC)"
	@echo "SM_BUILD = $(SM_BUILD)"
	@echo "SM_DIST  = $(SM_DIST)"
	@echo "LIBJS    = $(LIBJS)"
	@echo "LIBDIR   = $(LIBDIR)"
	@echo "DIST_ROOT = $(DIST_ROOT)"
	@echo "HEADER_ROOT = $(HEADER_ROOT)"
	@echo "INCLUDES    = $(INCLUDES)"
	@echo "JS_CONF_CAND = $(JS_CONF_CAND)"

print-curl:
	@echo "CURL_PKGCONFIG = $(CURL_PKGCONFIG)"
	@echo "CURL_CFLAGS    = $(CURL_CFLAGS)"
	@echo "CURL_LIBS      = $(CURL_LIBS)"

print-deps:
	@echo "=== Dependencies ==="
	@echo "CURL_PREFIX  = $(CURL_PREFIX)"
	@echo "CURL_CFLAGS  = $(CURL_CFLAGS)"
	@echo "CURL_LIBS    = $(CURL_LIBS)"
	@echo "JSON_PREFIX  = $(JSON_PREFIX)"
	@echo "JSON_CFLAGS  = $(JSON_CFLAGS)"
	@echo "SYS_LIBS     = $(SYS_LIBS)"

run: $(OUT)
	@if [ -z "$(FILE)" ]; then \
		echo "Use: make run FILE=path/to/script.jsc"; exit 1; \
	fi
	./$(OUT) "$(FILE)"

# Test sample JSC file
TEST_SAMPLE ?= samples/testdata/simple.jsc

test: test-suite test-golden
	@echo "All tests completed"

test-suite: $(OUT)
	@echo "=== Running Core Test Suite ==="
	./tests/test_suite.sh

test-samples: $(OUT)
	@echo "=== Testing Dedicated Test Samples ==="
	./tests/test_samples.sh

test-golden: $(OUT_TEST)
	@echo "=== Running Golden/Snapshot Tests ==="
	./tests/test_golden.sh

test-golden-update: $(OUT_TEST)
	@echo "=== Updating Golden Snapshots ==="
	./tests/test_golden.sh --generate

test-ollama: $(OUT)
	@echo "=== Running Ollama Test Suite ==="
	./tests/test_ollama.sh

test-basic: $(OUT)
	@echo "=== TEST: Basic analysis (no flags) ==="
	./$(OUT) "$(TEST_SAMPLE)"

test-inner: $(OUT)
	@echo "=== TEST: With inner function detection (default) ==="
	./$(OUT) "$(TEST_SAMPLE)"

test-resolve: $(OUT)
	@echo "=== TEST: With inner function resolution ==="
	./$(OUT) "$(TEST_SAMPLE)"

test-debug: $(OUT)
	@echo "=== TEST: Debug mode with full resolution ==="
	./$(OUT) --debug "$(TEST_SAMPLE)"

test-decompile: $(OUT)
	@echo "=== TEST: LLM decompilation with Ollama ==="
	./$(OUT) --decompile "$(TEST_SAMPLE)"
