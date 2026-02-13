package bytecode

// JOF_* constants define opcode operand format types
const (
	JOF_BYTE       = 0
	JOF_JUMP       = 1
	JOF_ATOM       = 2
	JOF_UINT16     = 3
	JOF_TABLESWITCH = 4
	JOF_QARG       = 6
	JOF_LOCAL      = 7
	JOF_DOUBLE     = 8
	JOF_UINT24     = 12
	JOF_UINT8      = 13
	JOF_INT32      = 14
	JOF_OBJECT     = 15
	JOF_REGEXP     = 17
	JOF_INT8       = 18
	JOF_ATOMOBJECT = 19
	JOF_SCOPECOORD = 21
	JOF_TYPEMASK   = 0x001f
)

// OpInfo holds metadata about a bytecode operation
type OpInfo struct {
	Name   string
	Length int8
	Format uint32
}

// JofType extracts the operand type from a format value
func JofType(format uint32) uint32 {
	return format & JOF_TYPEMASK
}

