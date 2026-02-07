package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zboralski/spidermonkey-dumper/sm33"
	"github.com/zboralski/spidermonkey-dumper/sm33/callgraph"
	"github.com/zboralski/spidermonkey-dumper/sm33/callgraph/render"
	"github.com/zboralski/spidermonkey-dumper/sm33/decompile"
	"github.com/zboralski/spidermonkey-dumper/sm33/disasm"
	"github.com/zboralski/spidermonkey-dumper/sm33/xdr"
)

func printDiag(d sm33.Diagnostic) {
	if d.Func != "" {
		fmt.Fprintf(os.Stderr, "diag [%s] %s @0x%x: %s\n", d.Kind, d.Func, d.Offset, d.Msg)
	} else {
		fmt.Fprintf(os.Stderr, "diag [%s] @0x%x: %s\n", d.Kind, d.Offset, d.Msg)
	}
}

func main() {
	decompileFlag := flag.Bool("decompile", false, "decompile bytecode via LLM")
	callgraphFlag := flag.Bool("callgraph", false, "generate callgraph SVG")
	cfgFlag := flag.Bool("controlflow", false, "generate control flow graph SVG")
	backend := flag.String("backend", "claude-code", "LLM backend: claude-code, codex")
	model := flag.String("model", "", "model name (backend-specific)")
	modeName := flag.String("mode", "strict", "decode mode: strict, besteffort")
	maxReadBytes := flag.Int("max-read-bytes", 0, "max bytes for a single XDR bytes() field (0 uses default)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: smdis [flags] <file.jsc>\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}

	var opt sm33.Options
	switch *modeName {
	case "strict":
		opt = sm33.DefaultOptions()
	case "besteffort":
		opt = sm33.Options{Mode: sm33.BestEffort}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown mode %q (use strict or besteffort)\n", *modeName)
		os.Exit(2)
	}
	opt.MaxReadBytes = *maxReadBytes

	path := flag.Arg(0)
	res, err := xdr.DecodeFileOpt(path, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, d := range res.Diags {
		printDiag(d)
	}

	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)

	// Callgraph mode
	if *callgraphFlag {
		dotPath, err := exec.LookPath("dot")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: graphviz not found (install with: brew install graphviz)\n")
			os.Exit(1)
		}

		g := callgraph.Build(res.Value)
		title := filepath.Base(path)
		if res.Value.Filename != "" {
			title = filepath.Base(res.Value.Filename)
		}
		dot := render.DOT(g, title)

		// Write .dot file
		dotFile := base + ".dot"
		if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", dotFile, err)
		}

		// Run dot to generate SVG and PNG
		for _, ext := range []string{"svg", "png"} {
			outFile := base + "." + ext
			args := []string{"-T" + ext, "-o", outFile, dotFile}
			if ext == "png" {
				args = []string{"-T" + ext, "-Gdpi=200", "-o", outFile, dotFile}
			}
			cmd := exec.Command(dotPath, args...)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: dot -T%s failed: %v\n", ext, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", outFile)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", dotFile)
		return
	}

	// CFG mode
	if *cfgFlag {
		dotPath, err := exec.LookPath("dot")
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: graphviz not found (install with: brew install graphviz)\n")
			os.Exit(1)
		}

		g := callgraph.BuildCFG(res.Value)
		title := filepath.Base(path)
		if res.Value.Filename != "" {
			title = filepath.Base(res.Value.Filename)
		}
		dot := render.DOTCFG(g, title)

		dotFile := base + ".cfg.dot"
		if err := os.WriteFile(dotFile, []byte(dot), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", dotFile, err)
		}

		for _, ext := range []string{"svg", "png"} {
			outFile := base + ".cfg." + ext
			args := []string{"-T" + ext, "-o", outFile, dotFile}
			if ext == "png" {
				args = []string{"-T" + ext, "-Gdpi=200", "-o", outFile, dotFile}
			}
			cmd := exec.Command(dotPath, args...)
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "error: dot -T%s failed: %v\n", ext, err)
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "wrote %s\n", outFile)
		}
		fmt.Fprintf(os.Stderr, "wrote %s\n", dotFile)
		return
	}

	disRes, err := disasm.DisasmTreeOpt(res.Value, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	for _, d := range disRes.Diags {
		printDiag(d)
	}

	out := disRes.Value
	fmt.Print(out)

	// Write .dis file alongside input
	disPath := base + ".dis"
	if err := os.WriteFile(disPath, []byte(out), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", disPath, err)
	}

	// Optional LLM decompilation
	if *decompileFlag {
		cfg := decompile.DefaultConfig()
		cfg.Backend = *backend
		cfg.Model = *model

		funcName := filepath.Base(base)

		js, err := decompile.Decompile(context.Background(), cfg, out, funcName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "decompile error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println(js)

		// Write .js file alongside input, include backend in name
		suffix := strings.ReplaceAll(cfg.Backend, "-", "")
		jsPath := base + "-" + suffix + ".js"
		if err := os.WriteFile(jsPath, []byte(js+"\n"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write %s: %v\n", jsPath, err)
		} else {
			fmt.Fprintf(os.Stderr, "wrote %s\n", jsPath)
		}
	}
}
