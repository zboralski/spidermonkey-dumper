package bytecode

import "testing"

func TestTruncatedBytecode(t *testing.T) {
	bc := []byte{0x06} // goto opcode, but no operand bytes
	if v, ok := GetJumpOffset(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated jump, got (%d, %v)", v, ok)
	}
	if v, ok := GetUint16(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated uint16, got (%d, %v)", v, ok)
	}
	if v, ok := GetUint24(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated uint24, got (%d, %v)", v, ok)
	}
	if v, ok := GetUint32Index(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated uint32, got (%d, %v)", v, ok)
	}
	if v, ok := GetInt8(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated int8, got (%d, %v)", v, ok)
	}
	if v, ok := GetInt32(bc, 0); ok || v != 0 {
		t.Errorf("expected (0, false) for truncated int32, got (%d, %v)", v, ok)
	}
}

func TestTableswitchAbsurdRange(t *testing.T) {
	// TABLESWITCH (0x46) with high-low+1 way too large
	bc := make([]byte, 13)
	bc[0] = 0x46 // tableswitch opcode
	// default offset = 0
	// low = 0
	// high = 0x7FFFFFFF (absurdly high)
	bc[9] = 0x7F
	bc[10] = 0xFF
	bc[11] = 0xFF
	bc[12] = 0xFF

	n := InstrLen(bc, 0, &OpcodeTableV33)
	if n != -1 {
		t.Errorf("expected -1 for absurd tableswitch range, got %d", n)
	}
}
