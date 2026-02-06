# sm33dis

Decode + disassemble SpiderMonkey 33 `.jsc` bytecode. Optional LLM-assisted “decompile” to JavaScript (best-effort).

## Build

```bash
go test ./...
go build ./cmd/sm33dis
```

## Usage

```bash
# Disassemble (strict mode by default)
./sm33dis path/to/file.jsc > out.dis

# Best-effort mode keeps going on malformed inputs and prints diagnostics to stderr
./sm33dis -mode=besteffort path/to/file.jsc > out.dis

# Disassemble + decompile via an LLM backend
./sm33dis -decompile -backend=claude-code samples/simple.jsc > /dev/null
./sm33dis -decompile -backend=codex samples/simple.jsc > /dev/null
```

Output files are written alongside the input: `file.dis` and (when `-decompile` is enabled) `file-<backend>.js`.

## Why This Exists (A Small RE Irony)

Some Cocosx games shipped SpiderMonkey `.jsc` bytecode **unencrypted**. That should make reversing easier, but in practice it often made it harder. The only thing available was an unfinished disassembly API inside the Firefox sources. It didn't support lambdas, nested functions, or most of the operand format.

This tool focuses on the boring part: reliably parsing the raw XDR format and turning it into stable disassembly we can build analyses on.

## Samples

See `samples/` for paired inputs/outputs: `.jsc` (input), `.dis` (disassembly), `*-claudecode.js` and `*-codex.js` (per-backend decompilations).

## Reference Source

Built from [SpiderMonkey 33](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/) (Firefox 33):

- [Opcodes.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/vm/Opcodes.h) — opcode definitions
- [jsopcode.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsopcode.h), [jsopcode.cpp](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsopcode.cpp) — opcode implementation
- [Xdr.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/vm/Xdr.h), [jsscript.cpp](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsscript.cpp) — XDR serialization
