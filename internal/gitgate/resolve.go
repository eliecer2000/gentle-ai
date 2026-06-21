package gitgate

// Resolve implements the 3-state resolution algorithm.
//
// Resolution matrix:
//   - strict_workflow=false → ModeOff (global kill switch)
//   - per-gate config absent and strict_workflow=true → ModeEnforce (safe default)
//   - per-gate config present → use that value
//   - sentinelConsumed and resolved mode == ModeEnforce → degrade to ModeWarn
//   - sentinelConsumed and resolved mode != ModeEnforce → no change
func Resolve(cfg Config, gate string, sentinelConsumed bool) GateMode {
	if !cfg.StrictWorkflow {
		return ModeOff
	}

	mode := ModeEnforce
	if cfg.Gates != nil {
		if m, ok := cfg.Gates[gate]; ok {
			mode = m
		}
	}

	if sentinelConsumed && mode == ModeEnforce {
		mode = ModeWarn
	}

	return mode
}
