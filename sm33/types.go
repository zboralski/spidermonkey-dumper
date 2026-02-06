package sm33

// Mode controls error handling behavior for decode and disassembly.
type Mode int

const (
	// Strict returns an error on the first structural invalidity.
	Strict Mode = iota
	// BestEffort continues with zero-values, collecting diagnostics.
	BestEffort
)

// Options configures decode and disassembly behavior.
type Options struct {
	Mode     Mode
	MaxSteps int // safety cap for loop iterations; 0 uses default
}

// DefaultOptions returns Strict mode with default step limit.
func DefaultOptions() Options {
	return Options{Mode: Strict}
}

// DefaultMaxSteps is the default safety cap for iteration loops.
const DefaultMaxSteps = 1 << 20

// EffectiveMaxSteps returns the effective step limit.
func (o Options) EffectiveMaxSteps() int {
	if o.MaxSteps <= 0 {
		return DefaultMaxSteps
	}
	return o.MaxSteps
}

// Diagnostic records one anomaly found during decode or disassembly.
type Diagnostic struct {
	Offset int    // byte offset where the issue occurred (within Func's bytecode)
	Kind   string // "truncated", "invalid", "overflow", "unknown_opcode", "clamped"
	Msg    string
	Func   string // function name, set when aggregating diagnostics from multiple scripts
}

// Result pairs a value with accumulated diagnostics.
type Result[T any] struct {
	Value T
	Diags []Diagnostic
}
