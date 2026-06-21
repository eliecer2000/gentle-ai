package gitgate

// GateMode represents the three-state enforcement mode for a git gate.
type GateMode string

const (
	// ModeEnforce causes the gate to block the operation when the check fails.
	ModeEnforce GateMode = "enforce"
	// ModeWarn allows the operation but emits a visible warning and log entry.
	ModeWarn GateMode = "warn"
	// ModeOff disables the gate entirely; the operation is always allowed silently.
	ModeOff GateMode = "off"
)

// GateResult holds the outcome of a gate check.
type GateResult struct {
	// Allowed reports whether the operation should proceed.
	Allowed bool
	// Mode is the effective mode that produced this result.
	Mode GateMode
	// Message is a human-readable explanation (non-empty on deny or warn).
	Message string
	// SentinelConsumed reports whether a break-glass sentinel was consumed.
	SentinelConsumed bool
}
