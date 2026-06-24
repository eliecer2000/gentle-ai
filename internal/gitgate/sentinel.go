package gitgate

import (
	"os"
	"path/filepath"
	"strings"
)

// SentinelPath returns the path for the break-glass sentinel file for the
// given gate under cwd. The sentinel is an empty file whose presence signals
// a one-time override request.
func SentinelPath(cwd, gate string) string {
	return filepath.Join(cwd, ".gentle-ai", "git-gate-override", gate)
}

// ConsumeSentinel checks whether the sentinel file at path exists and, if so,
// removes it atomically (consumed-once semantics). Returns consumed=true when
// the sentinel was present and successfully deleted. Returns consumed=false
// when the file does not exist. Returns an error only on unexpected OS failures.
func ConsumeSentinel(path string) (bool, error) {
	err := os.Remove(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// EnsureGitignored appends a .gentle-ai/ entry to <cwd>/.gitignore when
// it is not already present. Creates the file if it does not exist.
// This ensures sentinel files can never be accidentally committed.
func EnsureGitignored(cwd string) error {
	const entry = ".gentle-ai/"
	gitignorePath := filepath.Join(cwd, ".gitignore")

	data, err := os.ReadFile(gitignorePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	existing := string(data)
	for _, line := range strings.Split(existing, "\n") {
		if strings.TrimSpace(line) == entry {
			return nil // already present
		}
	}

	// Append with a leading newline when content already exists without trailing newline.
	var append string
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		append = "\n" + entry + "\n"
	} else {
		append = entry + "\n"
	}

	f, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(append)
	return err
}
