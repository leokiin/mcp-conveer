package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
# comment
PLAIN_KEY=plain_value
QUOTED_DOUBLE="double quoted"
QUOTED_SINGLE='single quoted'
  SPACES_AROUND  =  trimmed

ALREADY_SET=should_not_change
`
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("ALREADY_SET", "original")

	if err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}

	cases := []struct{ key, want string }{
		{"PLAIN_KEY", "plain_value"},
		{"QUOTED_DOUBLE", "double quoted"},
		{"QUOTED_SINGLE", "single quoted"},
		{"SPACES_AROUND", "trimmed"},
		{"ALREADY_SET", "original"},
	}
	for _, c := range cases {
		if got := os.Getenv(c.key); got != c.want {
			t.Errorf("key %s: got %q, want %q", c.key, got, c.want)
		}
	}
}

func TestLoad_MissingFile(t *testing.T) {
	if err := Load("/nonexistent/.env"); err != nil {
		t.Fatalf("expected nil for missing file, got: %v", err)
	}
}
