package xdr

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zboralski/spidermonkey-dumper/sm"
)

func TestBadMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	_, err := Decode(data)
	if err == nil {
		t.Fatal("expected error for bad magic")
	}
}

func TestEmptyInput(t *testing.T) {
	_, err := Decode(nil)
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestNegativeBytes(t *testing.T) {
	r := newReader([]byte{1, 2, 3}, sm.Strict, sm.MaxReadBytes)
	_, err := r.bytes(-1)
	if err == nil {
		t.Fatal("expected error for negative byte count")
	}
}

func TestNegativeBytesBestEffort(t *testing.T) {
	r := newReader([]byte{1, 2, 3}, sm.BestEffort, sm.MaxReadBytes)
	b, err := r.bytes(-1)
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if b == nil {
		t.Fatal("BestEffort negative bytes should return []byte{}, not nil")
	}
	if len(b) != 0 {
		t.Fatalf("expected empty slice, got %d bytes", len(b))
	}
	if len(r.diags) == 0 || r.diags[0].Kind != "invalid" {
		t.Errorf("expected invalid diagnostic, got: %+v", r.diags)
	}
}

func TestHugeBytesStrict(t *testing.T) {
	r := newReader(make([]byte, 100), sm.Strict, sm.MaxReadBytes)
	_, err := r.bytes(sm.MaxReadBytes + 1)
	if err == nil {
		t.Fatal("Strict should error on bytes exceeding MaxReadBytes")
	}
}

func TestHugeBytesBestEffort(t *testing.T) {
	r := newReader(make([]byte, 100), sm.BestEffort, sm.MaxReadBytes)
	b, err := r.bytes(sm.MaxReadBytes + 1)
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	// Clamped to MaxReadBytes, then truncated to available (100 bytes)
	if len(b) != 100 {
		t.Fatalf("expected 100 bytes (available data), got %d", len(b))
	}
	clamped := false
	for _, d := range r.diags {
		if d.Kind == "clamped" {
			clamped = true
		}
	}
	if !clamped {
		t.Error("expected clamped diagnostic")
	}
}

func TestStrictBadMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	_, err := DecodeOpt(data, sm.DefaultOptions())
	if err == nil {
		t.Fatal("Strict mode should error on bad magic")
	}
}

func TestBestEffortBadMagic(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00}
	res, err := DecodeOpt(data, sm.Options{Mode: sm.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if len(res.Diags) == 0 {
		t.Fatal("expected diagnostics for bad magic")
	}
	found := false
	for _, d := range res.Diags {
		if d.Kind == "invalid" && strings.Contains(d.Msg, "magic") {
			found = true
		}
	}
	if !found {
		t.Error("expected invalid magic diagnostic")
	}
}

func TestBestEffortTruncatedStream(t *testing.T) {
	// Valid magic + truncated script header
	data := make([]byte, 6)
	binary.LittleEndian.PutUint32(data, XdrMagicV33)
	data[4] = 0x01
	data[5] = 0x00

	res, err := DecodeOpt(data, sm.Options{Mode: sm.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	if len(res.Diags) == 0 {
		t.Fatal("expected truncation diagnostics")
	}
	t.Logf("got %d diagnostics", len(res.Diags))
}

func makeHugeCountInput(natoms uint32) []byte {
	data := make([]byte, 62)
	binary.LittleEndian.PutUint32(data[0:], XdrMagicV33)
	binary.LittleEndian.PutUint32(data[22:], natoms)
	return data
}

func TestHugeCountStrict(t *testing.T) {
	data := makeHugeCountInput(0xFFFFFFFF)
	_, err := DecodeOpt(data, sm.DefaultOptions())
	if err == nil {
		t.Fatal("Strict should error on huge natoms")
	}
}

func TestHugeCountBestEffort(t *testing.T) {
	data := makeHugeCountInput(0xFFFFFFFF)
	res, err := DecodeOpt(data, sm.Options{Mode: sm.BestEffort})
	if err != nil {
		t.Fatalf("BestEffort should not error: %v", err)
	}
	clamped := false
	for _, d := range res.Diags {
		if d.Kind == "clamped" {
			clamped = true
		}
	}
	if !clamped {
		t.Error("expected clamped diagnostic for huge natoms")
	}
}

func FuzzDecode(f *testing.F) {
	// Seed with real .jsc files from disasm testdata (v28 and v33)
	files, err := filepath.Glob("../disasm/testdata/*.jsc")
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	// Seed with v28 .jsc files from xdr testdata
	v28files, err := filepath.Glob("testdata/*.jsc")
	if err != nil {
		f.Fatal(err)
	}
	for _, path := range v28files {
		data, err := os.ReadFile(path)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// Strict mode: must not panic
		DecodeOpt(data, sm.DefaultOptions())

		// BestEffort mode: must not panic
		DecodeOpt(data, sm.Options{Mode: sm.BestEffort})
	})
}
