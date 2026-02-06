package disasm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zboralski/spidermonkey-dumper/sm33"
	"github.com/zboralski/spidermonkey-dumper/sm33/xdr"
)

func TestGoldenFiles(t *testing.T) {
	files, err := filepath.Glob("testdata/*.jsc")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no .jsc files found in testdata/")
	}

	for _, jscPath := range files {
		base := filepath.Base(jscPath)
		name := strings.TrimSuffix(base, filepath.Ext(base))
		t.Run(name, func(t *testing.T) {
			goldenPath := filepath.Join("testdata", name+".dis")
			golden, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			script, err := xdr.DecodeFile(jscPath)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			got := DisasmTree(script)
			if got != string(golden) {
				t.Errorf("output mismatch for %s", name)
				t.Logf("got length=%d, want length=%d", len(got), len(golden))
			}
		})
	}
}

func TestUnknownOpcodeStrict(t *testing.T) {
	s := &sm33.Script{Bytecode: []byte{0xFF}}
	_, err := DisasmScriptOpt(s, "test", false, sm33.DefaultOptions())
	if err == nil {
		t.Fatal("Strict should error on unknown opcode")
	}
}

func TestUnknownOpcodeBestEffort(t *testing.T) {
	s := &sm33.Script{Bytecode: []byte{0xFF}}
	res, err := DisasmScriptOpt(s, "test", false, sm33.Options{Mode: sm33.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if !strings.Contains(res.Value, "OP_0xFF") {
		t.Errorf("expected OP_0xFF in output, got: %s", res.Value)
	}
	if len(res.Diags) == 0 {
		t.Error("expected unknown_opcode diagnostic")
	}
}

func TestTruncatedOperandStrict(t *testing.T) {
	// goto (0x06) needs 5 bytes total, give it only 1
	s := &sm33.Script{Bytecode: []byte{0x06}}
	_, err := DisasmScriptOpt(s, "test", false, sm33.DefaultOptions())
	if err == nil {
		t.Fatal("Strict should error on truncated operand")
	}
}

func TestTruncatedOperandBestEffort(t *testing.T) {
	s := &sm33.Script{Bytecode: []byte{0x06}}
	res, err := DisasmScriptOpt(s, "test", false, sm33.Options{Mode: sm33.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if !strings.Contains(res.Value, "<truncated>") {
		t.Errorf("expected <truncated> in output, got: %s", res.Value)
	}
}

func TestInnerFunctionErrorStrict(t *testing.T) {
	// Script with an inner function whose bytecode is a truncated goto
	inner := &sm33.Script{Bytecode: []byte{0x06}} // truncated goto
	s := &sm33.Script{
		Bytecode: []byte{0x00}, // nop
		Objects: []*sm33.Object{{
			Kind:     sm33.CkJSFunction,
			Function: &sm33.Function{Name: "broken", Script: inner},
		}},
	}
	_, err := DisasmTreeOpt(s, sm33.DefaultOptions())
	if err == nil {
		t.Fatal("Strict should propagate error from inner function")
	}
}

func TestInnerFunctionErrorBestEffort(t *testing.T) {
	inner := &sm33.Script{Bytecode: []byte{0x06}} // truncated goto
	s := &sm33.Script{
		Bytecode: []byte{0x00}, // nop
		Objects: []*sm33.Object{{
			Kind:     sm33.CkJSFunction,
			Function: &sm33.Function{Name: "broken", Script: inner},
		}},
	}
	res, err := DisasmTreeOpt(s, sm33.Options{Mode: sm33.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if !strings.Contains(res.Value, "<truncated>") {
		t.Errorf("expected <truncated> in output, got: %s", res.Value)
	}
}

func TestBestEffortDiagnosticFunc(t *testing.T) {
	// Named inner function: diagnostics carry the function name.
	t.Run("named", func(t *testing.T) {
		inner := &sm33.Script{Bytecode: []byte{0x06}}
		s := &sm33.Script{
			Bytecode: []byte{0x00},
			Objects: []*sm33.Object{{
				Kind:     sm33.CkJSFunction,
				Function: &sm33.Function{Name: "broken", Script: inner},
			}},
		}
		res, err := DisasmTreeOpt(s, sm33.Options{Mode: sm33.BestEffort})
		if err != nil {
			t.Fatalf("BestEffort should not error: %v", err)
		}
		found := false
		for _, d := range res.Diags {
			if d.Func == "broken" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Func=\"broken\", got: %+v", res.Diags)
		}
	})

	// Anonymous inner function: diagnostics use "anon#N" for disambiguation.
	t.Run("anonymous", func(t *testing.T) {
		inner := &sm33.Script{Bytecode: []byte{0x06}}
		s := &sm33.Script{
			Bytecode: []byte{0x00},
			Objects: []*sm33.Object{{
				Kind:     sm33.CkJSFunction,
				Function: &sm33.Function{Name: "", Script: inner},
			}},
		}
		res, err := DisasmTreeOpt(s, sm33.Options{Mode: sm33.BestEffort})
		if err != nil {
			t.Fatalf("BestEffort should not error: %v", err)
		}
		found := false
		for _, d := range res.Diags {
			if d.Func == "anon#0" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Func=\"anon#0\", got: %+v", res.Diags)
		}
	})
}

func FuzzDisasm(f *testing.F) {
	// Seed with bytecode snippets from known opcodes
	seeds := [][]byte{
		{0x00},                         // nop
		{0x06, 0x00, 0x00, 0x00, 0x05}, // goto +5
		{0x07, 0x00, 0x00, 0x00, 0x05}, // ifeq +5
		{0x3C, 0x00, 0x00, 0x00, 0x01}, // getarg 1
		{0xFF},                         // unknown opcode
		{0x06},                         // truncated goto
		{0x46, 0x00, 0x00, 0x00, 0x0D, // tableswitch
			0x00, 0x00, 0x00, 0x00, // low=0
			0x00, 0x00, 0x00, 0x01, // high=1
			0x00, 0x00, 0x00, 0x0D}, // case offset
	}
	for _, s := range seeds {
		f.Add(s)
	}

	// Also seed with bytecode from real .jsc files
	files, err := filepath.Glob("testdata/*.jsc")
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		res, err := xdr.DecodeOpt(data, sm33.Options{Mode: sm33.BestEffort})
		if err == nil && res.Value != nil && len(res.Value.Bytecode) > 0 {
			f.Add(res.Value.Bytecode)
		}
	}

	f.Fuzz(func(t *testing.T, bc []byte) {
		s := &sm33.Script{Bytecode: bc}

		// Strict mode: must not panic
		DisasmScriptOpt(s, "fuzz", false, sm33.DefaultOptions())

		// BestEffort mode: must not panic
		DisasmScriptOpt(s, "fuzz", false, sm33.Options{Mode: sm33.BestEffort})
	})
}
