package postgres

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCollectUpMigrationFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := []string{
		"000002_more.up.sql",
		"000001_init.up.sql",
		"000001_init.down.sql",
		"README.md",
	}
	for _, name := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- test"), 0o644); err != nil {
			t.Fatalf("WriteFile(%q) error = %v", name, err)
		}
	}

	got, err := collectUpMigrationFiles(dir)
	if err != nil {
		t.Fatalf("collectUpMigrationFiles() error = %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if filepath.Base(got[0]) != "000001_init.up.sql" {
		t.Fatalf("got[0] = %q, want %q", filepath.Base(got[0]), "000001_init.up.sql")
	}
	if filepath.Base(got[1]) != "000002_more.up.sql" {
		t.Fatalf("got[1] = %q, want %q", filepath.Base(got[1]), "000002_more.up.sql")
	}
}
