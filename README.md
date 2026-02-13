# smdis

Decode + disassemble SpiderMonkey `.jsc` bytecode. Supports version 28 and 33 (auto-detected from XDR magic). Common in Cocos2d-x games. Optional LLM-assisted "decompile" to JavaScript (best-effort).

## Build

```bash
go test ./...
go build ./cmd/smdis
```

## Usage

```bash
# Disassemble (strict mode by default)
./smdis path/to/file.jsc > out.dis

# Best-effort mode keeps going on malformed inputs and prints diagnostics to stderr
./smdis -mode=besteffort path/to/file.jsc > out.dis

# Disassemble + decompile via an LLM backend
./smdis -decompile -backend=claude-code samples/simple.jsc > /dev/null
./smdis -decompile -backend=codex samples/simple.jsc > /dev/null

# Generate graphs (requires graphviz: `dot` on PATH)
./smdis -callgraph samples/simple.jsc
./smdis -cfg samples/simple.jsc
```

Output files are written alongside the input: `file.dis` and (when `-decompile` is enabled) `file-<backend>.js`.
Graph outputs (when enabled) are written alongside the input: `file.dot`/`file.svg`/`file.png` (callgraph) and `file.cfg.dot`/`file.cfg.svg`/`file.cfg.png` (control flow).

## Why This Exists (A Small RE Irony)

Some Cocosx games shipped SpiderMonkey `.jsc` bytecode **unencrypted**. That should make reversing easier, but in practice it often made it harder. The only thing available was an unfinished disassembly API inside the Firefox sources. It didn't support lambdas, nested functions, or most of the operand format.

This tool focuses on the boring part: reliably parsing the raw XDR format and turning it into stable disassembly we can build analyses on.

## Samples

Each `.jsc` input in `samples/` has paired outputs: `.dis` (disassembly), `*-claudecode.js` and `*-codex.js` (LLM decompilations), plus callgraph and control flow visualizations.

Regenerate all with `make samples`.

| Sample | Disassembly | Callgraph | Control Flow |
|--------|-------------|-----------|--------------|
| constants | [.dis](samples/constants.dis) | [svg](samples/constants.svg) | [svg](samples/constants.cfg.svg) |
| functions | [.dis](samples/functions.dis) | [svg](samples/functions.svg) | [svg](samples/functions.cfg.svg) |
| minimal | [.dis](samples/minimal.dis) | [svg](samples/minimal.svg) | [svg](samples/minimal.cfg.svg) |
| nested | [.dis](samples/nested.dis) | [svg](samples/nested.svg) | [svg](samples/nested.cfg.svg) |
| simple | [.dis](samples/simple.dis) | [svg](samples/simple.svg) | [svg](samples/simple.cfg.svg) |

## Reference Source

Built from [SpiderMonkey 33](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/) (Firefox 33) and [SpiderMonkey 28](https://github.com/nickygerritsen/spidermonkey-jsc-decompiler/tree/sm28) (Firefox 28):

- [Opcodes.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/vm/Opcodes.h) — opcode definitions
- [jsopcode.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsopcode.h), [jsopcode.cpp](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsopcode.cpp) — opcode implementation
- [Xdr.h](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/vm/Xdr.h), [jsscript.cpp](https://hg.mozilla.org/releases/mozilla-release/file/FIREFOX_33_0_RELEASE/js/src/jsscript.cpp) — XDR serialization
