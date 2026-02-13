package xdr

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"

	"github.com/zboralski/spidermonkey-dumper/sm"
)

// v28Encoder builds a v28 XDR byte stream for testing.
type v28Encoder struct {
	buf []byte
}

func (e *v28Encoder) u8(v uint8)  { e.buf = append(e.buf, v) }
func (e *v28Encoder) raw(b []byte) { e.buf = append(e.buf, b...) }

func (e *v28Encoder) u32(v uint32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, v)
	e.buf = append(e.buf, b...)
}

func (e *v28Encoder) cstring(s string) {
	e.buf = append(e.buf, []byte(s)...)
	e.buf = append(e.buf, 0)
}

// atomV28 writes a v28 atom: u32(length) + length*2 bytes UTF-16LE.
func (e *v28Encoder) atomV28(s string) {
	runes := []rune(s)
	u16s := utf16.Encode(runes)
	e.u32(uint32(len(u16s)))
	for _, c := range u16s {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, c)
		e.buf = append(e.buf, b...)
	}
}

// scriptSource writes a minimal ScriptSource with just a filename.
func (e *v28Encoder) scriptSource(filename string) {
	e.u8(0) // hasSource = false
	e.u8(1) // retrievable = true
	e.u8(0) // haveSourceMap = false
	e.u8(0) // haveDisplayURL = false
	if filename != "" {
		e.u8(1) // haveFilename = true
		e.cstring(filename)
	} else {
		e.u8(0) // haveFilename = false
	}
}

type v28TryNote struct {
	Kind       uint8
	StackDepth uint16
	Start      uint32
	Length     uint32
}

type v28BlockScope struct {
	Index, Start, Length, Parent uint32
}

// v28Obj is a test-only inner object for the v28 encoder.
type v28Obj struct {
	Kind     uint32     // sm.CkJSFunction, CkBlockObject, etc.
	FuncName string     // for CkJSFunction: function name (empty = anonymous)
	Inner    *v28Script // for CkJSFunction: non-lazy inner script body
}

type v28Script struct {
	Nargs       uint16
	Nvars       uint32
	Nfixed      uint16
	Version     uint16
	MainOffset  uint32
	Filename    string
	SourceStart uint32
	SourceEnd   uint32
	Lineno      uint32
	Nslots      uint16
	StaticLevel uint16
	Bytecode    []byte
	Srcnotes    []byte
	Atoms       []string
	Bindings    []string
	TryNotes    []v28TryNote
	BlockScopes []v28BlockScope
	Objects     []v28Obj
}

func encodeV28(s *v28Script) []byte {
	return encodeV28Ext(s, true)
}

func encodeV28Ext(s *v28Script, withMagic bool) []byte {
	e := &v28Encoder{}

	if withMagic {
		e.u32(XdrMagicV28)
	}

	// argsVars packed: (nargs << 16) | nvars
	e.u32(uint32(s.Nargs)<<16 | s.Nvars)

	// length (bytecode length)
	e.u32(uint32(len(s.Bytecode)))

	// prologLength (mainOffset)
	e.u32(s.MainOffset)

	// version packed: (nfixed << 16) | version
	e.u32(uint32(s.Nfixed)<<16 | uint32(s.Version))

	// natoms
	e.u32(uint32(len(s.Atoms)))

	// nsrcnotes
	e.u32(uint32(len(s.Srcnotes)))

	// nconsts, nobjects, nregexps
	e.u32(0)
	e.u32(uint32(len(s.Objects)))
	e.u32(0)

	// ntrynotes
	e.u32(uint32(len(s.TryNotes)))

	// nblockscopes
	e.u32(uint32(len(s.BlockScopes)))

	// nTypeSets, funLength
	e.u32(0)
	e.u32(0)

	// scriptBits — set OwnSource if we have a filename
	bits := uint32(0)
	if s.Filename != "" {
		bits |= 1 << sbOwnSource
	}
	e.u32(bits)

	// Bindings (nargs + nvars atoms + descriptors)
	nameCount := uint32(s.Nargs) + s.Nvars
	for i := uint32(0); i < nameCount; i++ {
		if int(i) < len(s.Bindings) {
			e.atomV28(s.Bindings[i])
		} else {
			e.atomV28("")
		}
	}
	for i := uint32(0); i < nameCount; i++ {
		e.u8(0)
	}

	// ScriptSource (if OwnSource)
	if s.Filename != "" {
		e.scriptSource(s.Filename)
	}

	// Source location
	e.u32(s.SourceStart)
	e.u32(s.SourceEnd)
	e.u32(s.Lineno)

	// nslots packed: (staticLevel << 16) | nslots
	e.u32(uint32(s.StaticLevel)<<16 | uint32(s.Nslots))

	// Bytecode
	e.raw(s.Bytecode)

	// Srcnotes
	e.raw(s.Srcnotes)

	// Atoms (v28 format: u32 length + length*2 UTF-16LE)
	for _, atom := range s.Atoms {
		e.atomV28(atom)
	}

	// Objects
	for _, obj := range s.Objects {
		e.u32(obj.Kind)
		switch obj.Kind {
		case sm.CkJSFunction:
			e.u32(0) // enclosingScopeIndex
			firstword := uint32(0)
			if obj.FuncName != "" {
				firstword |= 1 // hasAtom
			}
			e.u32(firstword)
			if obj.FuncName != "" {
				e.atomV28(obj.FuncName)
			}
			e.u32(0) // flagsword: nargs=0, flags=0
			if obj.Inner != nil {
				e.raw(encodeV28Ext(obj.Inner, false))
			}
		}
	}

	// TryNotes (reverse order)
	for i := len(s.TryNotes) - 1; i >= 0; i-- {
		tn := s.TryNotes[i]
		// kindAndDepth packed: (kind << 16) | stackDepth
		e.u32(uint32(tn.Kind)<<16 | uint32(tn.StackDepth))
		e.u32(tn.Start)
		e.u32(tn.Length)
	}

	// Block scopes
	for _, bs := range s.BlockScopes {
		e.u32(bs.Index)
		e.u32(bs.Start)
		e.u32(bs.Length)
		e.u32(bs.Parent)
	}

	return e.buf
}

func TestV28RoundTripMinimal(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00}, // nop
		Srcnotes: []byte{0x00},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if s.Nargs != 0 {
		t.Errorf("nargs = %d, want 0", s.Nargs)
	}
	if s.Nvars != 0 {
		t.Errorf("nvars = %d, want 0", s.Nvars)
	}
	if len(s.Bytecode) != 1 || s.Bytecode[0] != 0x00 {
		t.Errorf("bytecode = %x, want [00]", s.Bytecode)
	}
	if len(s.Atoms) != 0 {
		t.Errorf("atoms = %v, want []", s.Atoms)
	}
}

func TestV28RoundTripFilename(t *testing.T) {
	data := encodeV28(&v28Script{
		Filename:    "src/game.js",
		Bytecode:    []byte{0x00},
		Srcnotes:    []byte{0x00},
		SourceStart: 10,
		SourceEnd:   200,
		Lineno:      42,
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if s.Filename != "src/game.js" {
		t.Errorf("filename = %q, want %q", s.Filename, "src/game.js")
	}
	if s.SourceStart != 10 {
		t.Errorf("sourceStart = %d, want 10", s.SourceStart)
	}
	if s.SourceEnd != 200 {
		t.Errorf("sourceEnd = %d, want 200", s.SourceEnd)
	}
	if s.Lineno != 42 {
		t.Errorf("lineno = %d, want 42", s.Lineno)
	}
}

func TestV28RoundTripAtoms(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00},
		Srcnotes: []byte{0x00},
		Atoms:    []string{"hello", "world", "test123"},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := []string{"hello", "world", "test123"}
	if len(s.Atoms) != len(want) {
		t.Fatalf("atoms len = %d, want %d", len(s.Atoms), len(want))
	}
	for i, w := range want {
		if s.Atoms[i] != w {
			t.Errorf("atoms[%d] = %q, want %q", i, s.Atoms[i], w)
		}
	}
}

func TestV28RoundTripUnicodeAtoms(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00},
		Srcnotes: []byte{0x00},
		Atoms:    []string{"日本語", "emoji🎮", "café"},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	want := []string{"日本語", "emoji🎮", "café"}
	if len(s.Atoms) != len(want) {
		t.Fatalf("atoms len = %d, want %d", len(s.Atoms), len(want))
	}
	for i, w := range want {
		if s.Atoms[i] != w {
			t.Errorf("atoms[%d] = %q, want %q", i, s.Atoms[i], w)
		}
	}
}

func TestV28RoundTripBindings(t *testing.T) {
	data := encodeV28(&v28Script{
		Nargs:    2,
		Nvars:    1,
		Bytecode: []byte{0x00},
		Srcnotes: []byte{0x00},
		Bindings: []string{"arg0", "arg1", "localVar"},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if s.Nargs != 2 {
		t.Errorf("nargs = %d, want 2", s.Nargs)
	}
	if s.Nvars != 1 {
		t.Errorf("nvars = %d, want 1", s.Nvars)
	}
	want := []string{"arg0", "arg1", "localVar"}
	if len(s.Bindings) != len(want) {
		t.Fatalf("bindings len = %d, want %d", len(s.Bindings), len(want))
	}
	for i, w := range want {
		if s.Bindings[i] != w {
			t.Errorf("bindings[%d] = %q, want %q", i, s.Bindings[i], w)
		}
	}
}

func TestV28RoundTripTryNotes(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00, 0x00, 0x00, 0x00, 0x00},
		Srcnotes: []byte{0x00},
		TryNotes: []v28TryNote{
			{Kind: 0, StackDepth: 1, Start: 0, Length: 5},
			{Kind: 1, StackDepth: 2, Start: 1, Length: 3},
		},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(s.TryNotes) != 2 {
		t.Fatalf("trynotes len = %d, want 2", len(s.TryNotes))
	}

	// First try note
	tn := s.TryNotes[0]
	if tn.Kind != 0 {
		t.Errorf("trynote[0].kind = %d, want 0", tn.Kind)
	}
	if tn.StackDepth != 1 {
		t.Errorf("trynote[0].stackDepth = %d, want 1", tn.StackDepth)
	}
	if tn.Start != 0 {
		t.Errorf("trynote[0].start = %d, want 0", tn.Start)
	}
	if tn.Length != 5 {
		t.Errorf("trynote[0].length = %d, want 5", tn.Length)
	}

	// Second try note
	tn = s.TryNotes[1]
	if tn.Kind != 1 {
		t.Errorf("trynote[1].kind = %d, want 1", tn.Kind)
	}
	if tn.StackDepth != 2 {
		t.Errorf("trynote[1].stackDepth = %d, want 2", tn.StackDepth)
	}
}

func TestV28RoundTripBlockScopes(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00},
		Srcnotes: []byte{0x00},
		BlockScopes: []v28BlockScope{
			{Index: 0, Start: 0, Length: 10, Parent: 0xFFFFFFFF},
			{Index: 1, Start: 5, Length: 3, Parent: 0},
		},
	})

	// Should decode without error — block scopes are skipped but must be parseable.
	_, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestV28RoundTripPackedFields(t *testing.T) {
	data := encodeV28(&v28Script{
		Nargs:       3,
		Nvars:       7,
		Nfixed:      5,
		Version:     28,
		MainOffset:  0,
		Nslots:      10,
		StaticLevel: 2,
		Bytecode:    []byte{0x00},
		Srcnotes:    []byte{0x00},
		Bindings:    []string{"a", "b", "c", "v0", "v1", "v2", "v3", "v4", "v5", "v6"},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if s.Nargs != 3 {
		t.Errorf("nargs = %d, want 3", s.Nargs)
	}
	if s.Nvars != 7 {
		t.Errorf("nvars = %d, want 7", s.Nvars)
	}
	if s.Version != 28 {
		t.Errorf("version = %d, want 28", s.Version)
	}
	if s.Nslots != 10 {
		t.Errorf("nslots = %d, want 10", s.Nslots)
	}
	if s.StaticLevel != 2 {
		t.Errorf("staticLevel = %d, want 2", s.StaticLevel)
	}
}

func TestV28MagicDetection(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00},
		Srcnotes: []byte{0x00},
	})

	magic := binary.LittleEndian.Uint32(data[:4])
	if magic != XdrMagicV28 {
		t.Errorf("magic = 0x%08x, want 0x%08x", magic, XdrMagicV28)
	}

	ver := sm.DetectVersion(magic)
	if ver != sm.Version28 {
		t.Errorf("DetectVersion = %d, want %d", ver, sm.Version28)
	}
}

// TestWriteV28Goldens generates v28 .jsc golden files in testdata.
// Run with: UPDATE_GOLDENS=1 go test -run TestWriteV28Goldens -v
func TestWriteV28Goldens(t *testing.T) {
	if os.Getenv("UPDATE_GOLDENS") == "" {
		t.Skip("set UPDATE_GOLDENS=1 to regenerate golden files")
	}

	dir := "testdata"
	os.MkdirAll(dir, 0o755)

	goldens := map[string][]byte{
		"minimal_v28.jsc": encodeV28(&v28Script{
			Bytecode: []byte{0x00}, // nop
			Srcnotes: []byte{0x00},
		}),
		"atoms_v28.jsc": encodeV28(&v28Script{
			// name "hello" (opcode 59=name, 5 bytes, JOF_ATOM with index 0)
			Bytecode: []byte{0x3B, 0x00, 0x00, 0x00, 0x00},
			Srcnotes: []byte{0x00},
			Atoms:    []string{"hello"},
			Filename: "test.js",
			Lineno:   1,
		}),
		"locals_v28.jsc": encodeV28(&v28Script{
			Nvars: 1,
			// getlocal 0 (opcode 86=0x56, 3 bytes, uint16 operand)
			// return (opcode 5=0x05, 1 byte)
			Bytecode: []byte{0x56, 0x00, 0x00, 0x05},
			Srcnotes: []byte{0x00},
			Bindings: []string{"x"},
			Filename: "locals.js",
			Lineno:   1,
		}),
	}

	for name, data := range goldens {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		t.Logf("wrote %s (%d bytes)", path, len(data))
	}
}

func TestV28RoundTripInnerFunction(t *testing.T) {
	data := encodeV28(&v28Script{
		Bytecode: []byte{0x00}, // nop
		Srcnotes: []byte{0x00},
		Objects: []v28Obj{{
			Kind:     sm.CkJSFunction,
			FuncName: "inner",
			Inner: &v28Script{
				Bytecode: []byte{0x05}, // return
				Srcnotes: []byte{0x00},
			},
		}},
	})

	s, err := Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(s.Objects) != 1 {
		t.Fatalf("objects len = %d, want 1", len(s.Objects))
	}
	obj := s.Objects[0]
	if obj.Kind != sm.CkJSFunction {
		t.Errorf("object kind = %d, want %d (CkJSFunction)", obj.Kind, sm.CkJSFunction)
	}
	if obj.Function == nil {
		t.Fatal("object.Function is nil")
	}
	if obj.Function.Name != "inner" {
		t.Errorf("function name = %q, want %q", obj.Function.Name, "inner")
	}
	if obj.Function.Script == nil {
		t.Fatal("function.Script is nil")
	}
	if len(obj.Function.Script.Bytecode) != 1 || obj.Function.Script.Bytecode[0] != 0x05 {
		t.Errorf("inner bytecode = %x, want [05]", obj.Function.Script.Bytecode)
	}
}
