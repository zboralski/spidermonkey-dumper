package sm

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
	Mode Mode

	// MaxSteps is a safety cap for loop iterations; 0 uses DefaultMaxSteps.
	MaxSteps int

	// MaxReadBytes caps any single bytes() allocation during XDR decode; 0 uses sm.MaxReadBytes.
	// This is a DoS/OOM guard. Larger caps can be necessary for real-world .jsc files.
	MaxReadBytes int
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

// EffectiveMaxReadBytes returns the effective bytes() cap for decode.
func (o Options) EffectiveMaxReadBytes() int {
	if o.MaxReadBytes <= 0 {
		return MaxReadBytes
	}
	return o.MaxReadBytes
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
