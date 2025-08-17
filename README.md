# SpiderMonkey Dumper

A reverse engineering tool for analyzing JavaScript bytecode files (.jsc) compiled with SpiderMonkey engine. Provides clean disassembly output with optional LLM-powered decompilation.

This tool is most useful for working with unencrypted .jsc files. Ironically, these are often harder to reverse than the encrypted ones, because until now there has been no proper SpiderMonkey bytecode disassembler. Encrypted .jsc files in Cocos2d-x are usually more straightforward to deal with: the decryption key is embedded in the compiled Cocos2d-x library, and once decrypted the .jsc yields the original JavaScript source code directly.

## Features

- Bytecode disassembly for SpiderMonkey JSC files
- Automatic lambda/property mapping
- Optional JavaScript reconstruction via Ollama

## Quick Start

### 1. Install Dependencies

```bash
./install-deps.sh
```

This installs:
- `autoconf213` - For SpiderMonkey build
- `nspr` - Netscape Portable Runtime (required by SpiderMonkey)  
- `curl` - HTTP client for LLM integration
- `nlohmann-json` - JSON library for C++

### 2. Build SpiderMonkey (First Time Only)

```bash
./buildSpiderMonkey.sh
```

### 3. Build the Dumper

```bash
make
```

### 4. Analyze JSC Files

```bash
# Fast disassembly (default)
./dumper file.jsc

# With LLM decompilation (requires Ollama)
./dumper --decompile file.jsc
```

## LLM Decompilation Setup

For advanced decompilation features, install Ollama:

```bash
brew install ollama
ollama pull gpt-oss:20b
```


## Usage Examples

### Basic Analysis
```bash
./dumper samples/script.jsc
```
Output:
- Clean bytecode disassembly to console
- `script.dis` file with plain disassembly

### LLM Decompilation
```bash
./dumper --decompile samples/script.jsc
```
Output:
- `script.dis` - Clean disassembly
- `script.js` - LLM-generated JavaScript code
- Console output with both disassembly and decompilation

### Advanced Options
```bash
# Debug mode with verbose output
./dumper --debug --decompile script.jsc

# Custom LLM model
./dumper --decompile --ollama-model "codellama" script.jsc

# Custom Ollama host
./dumper --decompile --ollama-host "http://localhost:11434" script.jsc

# Show line numbers and enable object analysis
./dumper --lines --objects script.jsc
```

## Command Line Options

| Flag | Description |
|------|-------------|
| `--decompile` | Enable LLM decompilation (requires Ollama) |
| `--debug` | Enable verbose debug output |
| `--inner` | Enable inner function detection |
| `--resolve-inner` | Resolve inner function names to properties |
| `--objects` | Show detailed object analysis |
| `--lines` | Show JavaScript source line numbers |
| `--color`/`--no-color` | Force enable/disable colored output |
| `--ollama-host URL` | Custom Ollama server URL |
| `--ollama-model NAME` | Custom LLM model (default: gpt-oss:20b) |

## Output Files

The dumper generates multiple output files:

- **`<filename>.dis`** - Clean, plain-text disassembly suitable for further analysis
- **`<filename>.js`** - LLM-generated JavaScript code (when using `--decompile`)

## Building from Source

### Prerequisites
- macOS with Xcode Command Line Tools
- Homebrew package manager
- Ollama (optional, for LLM features)

### Build Process
1. Install dependencies: `./install-deps.sh`
2. Build SpiderMonkey: `./buildSpiderMonkeyArm64.sh`
3. Build dumper: `make`

### Troubleshooting

If the build fails to find libcurl:
```bash
CURL_PREFIX="$(brew --prefix curl)"
CXXFLAGS="-I$CURL_PREFIX/include" LDFLAGS="-L$CURL_PREFIX/lib" make
```

If decompilation fails or hangs, check Ollama server logs:
```bash
# View recent Ollama server activity and errors
cat ~/.ollama/logs/server.log | tail -50

# Look for these common issues:
# - "truncating input prompt" (disassembly too large)
# - HTTP 500 errors (server overloaded)
# - "context canceled" (client timeout working correctly)
```

## Roadmap & Version Support

The current release of **dumper** targets **SpiderMonkey 33.1.1** as used in
**Cocos2d-JS / Cocos2d-x v3.17**. This lets you reverse unencrypted `.jsc`
files from that runtime.

Our roadmap is to extend support to additional SpiderMonkey builds used in
other Cocos2d-x releases as we go.

➡️ If you need a specific Cocos2d-x or SpiderMonkey version supported, please
open an **issue** in this repository with the version number and context. This
will help us prioritize and test against real-world use cases.

## License

This project uses code from Mozilla's SpiderMonkey engine and is licensed under the Mozilla Public License 2.0. See `LICENSE` for details.

## Contributing

1. Follow existing code style and patterns
2. Test with sample JSC files before submitting
3. Update documentation for new features
4. Ensure compatibility with SpiderMonkey 33.1.1