#!/bin/bash
# SpiderMonkey Dumper - Dependencies Installation Script
# Installs required dependencies via Homebrew

set -e

echo "Installing SpiderMonkey Dumper dependencies..."

# Check if Homebrew is installed
if ! command -v brew &> /dev/null; then
    echo "Error: Homebrew is not installed. Please install it first:"
    echo "  /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
    exit 1
fi

# Core dependencies for building SpiderMonkey dumper
echo "Installing core dependencies..."
brew install autoconf213 || true  # For SpiderMonkey build
brew install mercurial || true    # Required for SpiderMonkey source checkout
brew install nspr || true         # Netscape Portable Runtime (required by SpiderMonkey)
brew install nlohmann-json || true  # Header-only JSON library for C++

# HTTP client library for LLM integration
echo "Installing libcurl for LLM decompilation support..."
brew install curl || true

echo "Dependencies installed successfully!"
echo
echo "Optional: Install Ollama for LLM decompilation:"
echo "  brew install ollama"
echo "  ollama pull gpt-oss:20b"
echo
echo "To build the dumper:"
echo "  make"
echo
echo "LLM decompilation support is now enabled by default."
echo "If the build cannot find curl, try:" 
CURL_PREFIX=\"$(brew --prefix curl 2>/dev/null)\"
echo "  CXXFLAGS=\"-I\$CURL_PREFIX/include\" LDFLAGS=\"-L\$CURL_PREFIX/lib\" make"
echo
echo "Usage:"
echo "  ./dumper --decompile file.jsc     # emits <file>.dis (disassembly) and <file>.js (LLM decompile)"