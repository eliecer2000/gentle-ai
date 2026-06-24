package gitgate

import (
	"bufio"
	"os"
	"strings"
)

// Config holds the parsed gitgate configuration from openspec/config.yaml.
type Config struct {
	// StrictWorkflow is the global kill switch for all gates.
	// When false, all gates return ModeOff regardless of per-gate settings.
	StrictWorkflow bool
	// Gates maps gate names (e.g. "branch-base") to their configured GateMode.
	// Absent keys default to ModeEnforce when StrictWorkflow is true.
	Gates map[string]GateMode
	// SDD holds the SDD delivery configuration used by the sequential-pr gate.
	SDD SddConfig
}

// SddConfig holds SDD-specific config from the sdd: block in openspec/config.yaml.
type SddConfig struct {
	DeliveryStrategy    string
	ChainStrategy       string
	CurrentParentBranch string
	TaskBranches        []string
}

// ReadConfig parses a minimal subset of openspec/config.yaml without introducing
// a third-party YAML dependency. It reads only the keys needed by the gitgate
// package: strict_workflow, strict_workflow_gates:, and sdd:.
//
// Missing file → zero Config, no error (fail-open: a missing config must never
// block the agent). Parse errors on individual lines are silently skipped so
// that unrecognized or future YAML constructs do not break the gate.
func ReadConfig(path string) (Config, error) {
	cfg := Config{
		Gates: make(map[string]GateMode),
	}

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	defer f.Close()

	// Parsing state.
	const (
		stateRoot        = iota
		stateGates       // inside strict_workflow_gates: block
		stateSDD         // inside sdd: block
		stateSDDBranches // inside sdd.task_branches: block
	)
	state := stateRoot

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		// Skip comments and blank lines.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect block entries (indented lines) vs root keys.
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))

		switch state {
		case stateGates:
			if indent == 0 {
				// Left the gates block; fall through to root handling.
				state = stateRoot
			} else {
				// Parse "  gate-name: mode"
				if k, v, ok := splitKV(trimmed); ok {
					cfg.Gates[k] = GateMode(v)
				}
				continue
			}

		case stateSDD:
			if indent == 0 {
				state = stateRoot
			} else if trimmed == "task_branches:" {
				state = stateSDDBranches
				continue
			} else {
				if k, v, ok := splitKV(trimmed); ok {
					switch k {
					case "delivery_strategy":
						cfg.SDD.DeliveryStrategy = v
					case "chain_strategy":
						cfg.SDD.ChainStrategy = v
					case "current_parent_branch":
						cfg.SDD.CurrentParentBranch = v
					}
				}
				continue
			}

		case stateSDDBranches:
			if indent == 0 {
				state = stateRoot
			} else if strings.HasPrefix(trimmed, "- ") {
				branch := strings.TrimPrefix(trimmed, "- ")
				branch = strings.Trim(branch, `"'`)
				if branch != "" {
					cfg.SDD.TaskBranches = append(cfg.SDD.TaskBranches, branch)
				}
				continue
			} else {
				// Ended branches block (different non-list key under sdd:).
				state = stateSDD
			}
		}

		// Root-level key handling.
		if state == stateRoot {
			if k, v, ok := splitKV(trimmed); ok {
				switch k {
				case "strict_workflow":
					cfg.StrictWorkflow = v == "true"
				}
			} else if strings.TrimRight(trimmed, ":") == "strict_workflow_gates" {
				state = stateGates
			} else if strings.TrimRight(trimmed, ":") == "sdd" {
				state = stateSDD
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// splitKV splits a "key: value" line into key and value strings.
// Returns ok=false when the line has no colon or has only a block-start colon.
func splitKV(line string) (k, v string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	k = strings.TrimSpace(line[:idx])
	v = strings.TrimSpace(line[idx+1:])
	// A bare "key:" with no value (block header) is not a k/v pair.
	if v == "" {
		return "", "", false
	}
	// Strip inline comments.
	if ci := strings.Index(v, " #"); ci >= 0 {
		v = strings.TrimSpace(v[:ci])
	}
	return k, v, true
}
