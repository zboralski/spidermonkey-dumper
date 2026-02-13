package sm

import "github.com/zboralski/spidermonkey-dumper/sm/bytecode"

// Version identifies a SpiderMonkey bytecode version.
type Version int

const (
	VersionUnknown Version = 0
	Version28      Version = 28
	Version33      Version = 33
)

// DetectVersion returns the SpiderMonkey version from an XDR magic number.
// Magic numbers follow the pattern 0xb973c0de - subtrahend.
func DetectVersion(magic uint32) Version {
	sub := uint32(0xb973c0de) - magic
	switch sub {
	case 156:
		return Version28
	case 178:
		return Version33
	default:
		return VersionUnknown
	}
}

// OpcodeTable returns the opcode table for the given version.
func OpcodeTable(v Version) *[256]bytecode.OpInfo {
	switch v {
	case Version28:
		return &bytecode.OpcodeTableV28
	case Version33:
		return &bytecode.OpcodeTableV33
	default:
		return nil
	}
}
