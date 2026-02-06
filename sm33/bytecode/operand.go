package bytecode

// Bytecode operand readers (big-endian, matching SM33 encoding).
// All return (value, ok) where ok=false means truncated input.

func GetUint16(bc []byte, off int) (uint16, bool) {
	if off+3 > len(bc) {
		return 0, false
	}
	return uint16(bc[off+1])<<8 | uint16(bc[off+2]), true
}

func GetUint24(bc []byte, off int) (uint32, bool) {
	if off+4 > len(bc) {
		return 0, false
	}
	return uint32(bc[off+1])<<16 | uint32(bc[off+2])<<8 | uint32(bc[off+3]), true
}

func GetUint32Index(bc []byte, off int) (uint32, bool) {
	if off+5 > len(bc) {
		return 0, false
	}
	return uint32(bc[off+1])<<24 | uint32(bc[off+2])<<16 | uint32(bc[off+3])<<8 | uint32(bc[off+4]), true
}

func GetJumpOffset(bc []byte, off int) (int32, bool) {
	if off+5 > len(bc) {
		return 0, false
	}
	return int32(bc[off+1])<<24 | int32(bc[off+2])<<16 | int32(bc[off+3])<<8 | int32(bc[off+4]), true
}

func GetInt8(bc []byte, off int) (int8, bool) {
	if off+2 > len(bc) {
		return 0, false
	}
	return int8(bc[off+1]), true
}

func GetInt32(bc []byte, off int) (int32, bool) {
	if off+5 > len(bc) {
		return 0, false
	}
	return int32(bc[off+1])<<24 | int32(bc[off+2])<<16 | int32(bc[off+3])<<8 | int32(bc[off+4]), true
}

// GetArgno reads argument number (uint16)
func GetArgno(bc []byte, off int) (uint16, bool) {
	return GetUint16(bc, off)
}

// GetLocalno reads local variable number (uint24)
func GetLocalno(bc []byte, off int) (uint32, bool) {
	return GetUint24(bc, off)
}

// InstrLen returns the byte length of the instruction at bc[off].
// Returns -1 for unknown/invalid opcodes.
func InstrLen(bc []byte, off int) int {
	if off >= len(bc) {
		return -1
	}
	op := bc[off]
	info := &Opcodes[op]
	if info.Length > 0 {
		return int(info.Length)
	}
	if info.Length == -1 {
		// TABLESWITCH: variable length
		if off+13 > len(bc) {
			return -1
		}
		lowVal := int32(bc[off+5])<<24 | int32(bc[off+6])<<16 | int32(bc[off+7])<<8 | int32(bc[off+8])
		highVal := int32(bc[off+9])<<24 | int32(bc[off+10])<<16 | int32(bc[off+11])<<8 | int32(bc[off+12])
		n := int(highVal) - int(lowVal) + 1
		if n < 0 {
			return -1
		}
		maxN := (len(bc) - (off + 13)) / 4
		if n > maxN {
			return -1
		}
		return 1 + 4 + 4 + 4 + n*4
	}
	return -1
}

// CollectLabels identifies bytecode offsets that are jump targets.
func CollectLabels(bc []byte) map[int]struct{} {
	labels := make(map[int]struct{})
	off := 0
	for off < len(bc) {
		op := bc[off]
		info := &Opcodes[op]
		jt := JofType(info.Format)

		if jt == JOF_JUMP {
			if jumpOff, ok := GetJumpOffset(bc, off); ok {
				target := off + int(jumpOff)
				if target >= 0 && target <= len(bc) {
					labels[target] = struct{}{}
				}
			}
		} else if jt == JOF_TABLESWITCH {
			if defOff, ok := GetJumpOffset(bc, off); ok {
				defTgt := off + int(defOff)
				if defTgt >= 0 && defTgt <= len(bc) {
					labels[defTgt] = struct{}{}
				}
			}
			if off+13 <= len(bc) {
				lowVal := int32(bc[off+5])<<24 | int32(bc[off+6])<<16 | int32(bc[off+7])<<8 | int32(bc[off+8])
				highVal := int32(bc[off+9])<<24 | int32(bc[off+10])<<16 | int32(bc[off+11])<<8 | int32(bc[off+12])
				n := int(highVal) - int(lowVal) + 1
				maxN := (len(bc) - (off + 13)) / 4
				if n < 0 || n > maxN {
					n = 0
				}
				base := off + 13
				for i := 0; i < n; i++ {
					joff := base + i*4
					if joff+4 <= len(bc) {
						caseOff := int32(bc[joff])<<24 | int32(bc[joff+1])<<16 | int32(bc[joff+2])<<8 | int32(bc[joff+3])
						tgt := off + int(caseOff)
						if tgt >= 0 && tgt <= len(bc) {
							labels[tgt] = struct{}{}
						}
					}
				}
			}
		}

		n := InstrLen(bc, off)
		if n <= 0 {
			break
		}
		off += n
	}
	return labels
}
