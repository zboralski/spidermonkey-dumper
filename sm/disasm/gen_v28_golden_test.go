package disasm

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zboralski/spidermonkey-dumper/sm"
	"github.com/zboralski/spidermonkey-dumper/sm/bytecode"
	"github.com/zboralski/spidermonkey-dumper/sm/xdr"
)

// TestGenerateV28Goldens generates .dis golden files for v28 .jsc test data.
// Run with: UPDATE_GOLDENS=1 go test -run TestGenerateV28Goldens -v
func TestGenerateV28Goldens(t *testing.T) {
	if os.Getenv("UPDATE_GOLDENS") == "" {
		t.Skip("set UPDATE_GOLDENS=1 to regenerate golden files")
	}

	files, err := filepath.Glob("testdata/*_v28.jsc")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no v28 .jsc files found in testdata/")
	}

	for _, jscPath := range files {
		data, err := os.ReadFile(jscPath)
		if err != nil {
			t.Fatalf("read %s: %v", jscPath, err)
		}

		magic := binary.LittleEndian.Uint32(data[:4])
		ver := sm.DetectVersion(magic)
		ops := sm.OpcodeTable(ver)
		if ops == nil {
			t.Fatalf("unknown version for %s (magic 0x%08x)", jscPath, magic)
		}

		script, err := xdr.Decode(data)
		if err != nil {
			t.Fatalf("decode %s: %v", jscPath, err)
		}

		res, err := DisasmTreeOpt(script, sm.DefaultOptions(), ops)
		if err != nil {
			t.Fatalf("disasm %s: %v", jscPath, err)
		}

		base := jscPath[:len(jscPath)-4] // trim .jsc
		disPath := base + ".dis"
		if err := os.WriteFile(disPath, []byte(res.Value), 0o644); err != nil {
			t.Fatalf("write %s: %v", disPath, err)
		}
		t.Logf("wrote %s (%d bytes)", disPath, len(res.Value))
		fmt.Print(res.Value)
	}
}

// TestV28Disasm verifies v28 .jsc files produce correct disassembly with v28 opcodes.
func TestV28Disasm(t *testing.T) {
	v28ops := &bytecode.OpcodeTableV28

	t.Run("getlocal_uint16", func(t *testing.T) {
		// v28 getlocal uses uint16 operand (3-byte instruction)
		s := &sm.Script{Bytecode: []byte{0x56, 0x00, 0x00}}
		res, err := DisasmScriptOpt(s, "test", false, sm.DefaultOptions(), v28ops)
		if err != nil {
			t.Fatalf("disasm: %v", err)
		}
		if res.Value == "" {
			t.Fatal("empty output")
		}
		// Should show "getlocal     0" (local index 0)
		if !strings.Contains(res.Value, "getlocal") {
			t.Errorf("expected getlocal in output:\n%s", res.Value)
		}
		if !strings.Contains(res.Value, " 0") {
			t.Errorf("expected local index 0 in output:\n%s", res.Value)
		}
	})

	t.Run("getlocal_nonzero", func(t *testing.T) {
		// v28 getlocal with local index 5 → operand bytes 0x00, 0x05 (big-endian uint16)
		s := &sm.Script{Bytecode: []byte{0x56, 0x00, 0x05}}
		res, err := DisasmScriptOpt(s, "test", false, sm.DefaultOptions(), v28ops)
		if err != nil {
			t.Fatalf("disasm: %v", err)
		}
		if !strings.Contains(res.Value, " 5") {
			t.Errorf("expected local index 5 in output:\n%s", res.Value)
		}
	})

	t.Run("setlocal_uint16", func(t *testing.T) {
		// v28 setlocal (opcode 87=0x57) with local index 1
		s := &sm.Script{Bytecode: []byte{0x57, 0x00, 0x01}}
		res, err := DisasmScriptOpt(s, "test", false, sm.DefaultOptions(), v28ops)
		if err != nil {
			t.Fatalf("disasm: %v", err)
		}
		if !strings.Contains(res.Value, "setlocal") {
			t.Errorf("expected setlocal in output:\n%s", res.Value)
		}
		if !strings.Contains(res.Value, " 1") {
			t.Errorf("expected local index 1 in output:\n%s", res.Value)
		}
	})

	t.Run("notearg_v28_only", func(t *testing.T) {
		// opcode 228 (0xE4) is notearg in v28, tostring in v33
		s := &sm.Script{Bytecode: []byte{0xE4}}
		res, err := DisasmScriptOpt(s, "test", false, sm.DefaultOptions(), v28ops)
		if err != nil {
			t.Fatalf("disasm: %v", err)
		}
		if !strings.Contains(res.Value, "notearg") {
			t.Errorf("expected notearg in output:\n%s", res.Value)
		}
	})
}

