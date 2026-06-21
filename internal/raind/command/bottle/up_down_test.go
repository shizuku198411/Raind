package bottle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveBottleFilePrefersBottleYaml(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("compose.yaml", []byte("bottle:\n  name: compose\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("bottle.yaml", []byte("bottle:\n  name: bottle\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBottleFile("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "bottle.yaml" {
		t.Fatalf("expected bottle.yaml, got %q", got)
	}
}

func TestResolveBottleFileFallsBackToComposeYaml(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.WriteFile("compose.yaml", []byte("bottle:\n  name: compose\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBottleFile("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "compose.yaml" {
		t.Fatalf("expected compose.yaml, got %q", got)
	}
}

func TestResolveBottleFileUsesExplicitPath(t *testing.T) {
	dir := t.TempDir()
	explicit := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(explicit, []byte("bottle:\n  name: custom\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveBottleFile(explicit)
	if err != nil {
		t.Fatal(err)
	}
	if got != explicit {
		t.Fatalf("expected explicit path, got %q", got)
	}
}

func TestResolveBottleFileMissing(t *testing.T) {
	dir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	_, err = resolveBottleFile("")
	if err == nil || !strings.Contains(err.Error(), "use -f") {
		t.Fatalf("expected missing file error, got %v", err)
	}
}

func TestReadBottleNameFromFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bottle.yaml")
	if err := os.WriteFile(file, []byte("bottle:\n  name: wordpress-mysql\nservices: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := readBottleNameFromFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got != "wordpress-mysql" {
		t.Fatalf("expected wordpress-mysql, got %q", got)
	}
}

func TestReadBottleNameFromFileRequiresBottleName(t *testing.T) {
	file := filepath.Join(t.TempDir(), "bottle.yaml")
	if err := os.WriteFile(file, []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := readBottleNameFromFile(file)
	if err == nil || !strings.Contains(err.Error(), "bottle.name is required") {
		t.Fatalf("expected bottle.name error, got %v", err)
	}
}
