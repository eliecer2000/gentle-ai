package gitgate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSentinelPath(t *testing.T) {
	got := SentinelPath("/some/project", "branch-base")
	want := filepath.Join("/some/project", ".gentle-ai", "git-gate-override", "branch-base")
	if got != want {
		t.Errorf("SentinelPath() = %q, want %q", got, want)
	}
}

func TestConsumeSentinel(t *testing.T) {
	t.Run("sentinel present: consumed=true, file deleted", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "override-gate")
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		consumed, err := ConsumeSentinel(path)
		if err != nil {
			t.Fatalf("ConsumeSentinel() error = %v", err)
		}
		if !consumed {
			t.Fatal("consumed = false, want true")
		}
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatal("sentinel file still exists after consume")
		}
	})

	t.Run("sentinel absent: consumed=false, no error", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-such-file")
		consumed, err := ConsumeSentinel(path)
		if err != nil {
			t.Fatalf("ConsumeSentinel() error = %v", err)
		}
		if consumed {
			t.Fatal("consumed = true, want false")
		}
	})

	t.Run("second call after consume: consumed=false (truly consumed-once)", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "once")
		if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		consumed1, err := ConsumeSentinel(path)
		if err != nil || !consumed1 {
			t.Fatalf("first ConsumeSentinel() consumed=%v err=%v", consumed1, err)
		}
		consumed2, err := ConsumeSentinel(path)
		if err != nil {
			t.Fatalf("second ConsumeSentinel() error = %v", err)
		}
		if consumed2 {
			t.Fatal("second consumed = true, want false")
		}
	})
}

func TestEnsureGitignored(t *testing.T) {
	t.Run("adds .gentle-ai/ when absent", func(t *testing.T) {
		dir := t.TempDir()
		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("*.log\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := EnsureGitignored(dir); err != nil {
			t.Fatalf("EnsureGitignored() error = %v", err)
		}
		data, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), ".gentle-ai/") {
			t.Fatalf(".gitignore missing .gentle-ai/ entry:\n%s", data)
		}
	})

	t.Run("idempotent: does not duplicate", func(t *testing.T) {
		dir := t.TempDir()
		gitignore := filepath.Join(dir, ".gitignore")
		if err := os.WriteFile(gitignore, []byte("*.log\n.gentle-ai/\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := EnsureGitignored(dir); err != nil {
			t.Fatalf("EnsureGitignored() error = %v", err)
		}
		data, err := os.ReadFile(gitignore)
		if err != nil {
			t.Fatal(err)
		}
		count := strings.Count(string(data), ".gentle-ai/")
		if count != 1 {
			t.Fatalf(".gentle-ai/ appears %d times, want 1:\n%s", count, data)
		}
	})

	t.Run("creates .gitignore when absent", func(t *testing.T) {
		dir := t.TempDir()
		if err := EnsureGitignored(dir); err != nil {
			t.Fatalf("EnsureGitignored() error = %v", err)
		}
		data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), ".gentle-ai/") {
			t.Fatalf(".gitignore missing .gentle-ai/ entry:\n%s", data)
		}
	})
}
