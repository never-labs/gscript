package architecture_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestExampleDialectAndPackageManagedNamingGuards(t *testing.T) {
	root := findRepoRoot(t)

	dialectDir := filepath.Join(root, "examples", "dialects")
	entries, err := os.ReadDir(dialectDir)
	if err != nil {
		t.Fatalf("read examples/dialects: %v", err)
	}
	dialectName := regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*\.leia$`)
	dialectCount := 0
	for _, entry := range entries {
		if entry.IsDir() {
			t.Fatalf("examples/dialects must stay flat; found directory %s", entry.Name())
		}
		if filepath.Ext(entry.Name()) != ".leia" {
			t.Fatalf("examples/dialects must contain runnable .leia files only; found %s", entry.Name())
		}
		if !dialectName.MatchString(entry.Name()) {
			t.Fatalf("dialect example %s must use snake_case .leia naming", entry.Name())
		}
		dialectCount++
	}
	if dialectCount == 0 {
		t.Fatal("examples/dialects must contain at least one runnable dialect example")
	}

	var packageManagedDirs []string
	err = filepath.WalkDir(filepath.Join(root, "examples"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		switch entry.Name() {
		case "package-managed", "packageManaged", "packagemanaged":
			rel, relErr := filepath.Rel(root, path)
			if relErr != nil {
				return relErr
			}
			t.Fatalf("package-managed examples must use package_managed directory naming; found %s", filepath.ToSlash(rel))
		case "package_managed":
			packageManagedDirs = append(packageManagedDirs, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk examples: %v", err)
	}
	if len(packageManagedDirs) == 0 {
		t.Fatal("examples must keep at least one package_managed import example")
	}

	parentName := regexp.MustCompile(`^[a-z0-9]+(?:_[a-z0-9]+)*$`)
	for _, dir := range packageManagedDirs {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatal(err)
		}
		parent := filepath.Base(filepath.Dir(dir))
		if !parentName.MatchString(parent) {
			t.Fatalf("%s parent directory must use snake_case naming", filepath.ToSlash(rel))
		}
		for _, required := range []string{"leia.mod", "main.leia"} {
			if _, err := os.Stat(filepath.Join(dir, required)); err != nil {
				t.Fatalf("%s must contain %s: %v", filepath.ToSlash(rel), required, err)
			}
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", filepath.ToSlash(rel), err)
		}
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".leia" {
				continue
			}
			if entry.Name() != "main.leia" {
				t.Fatalf("%s package-managed import example must use main.leia as its runnable entrypoint, found %s", filepath.ToSlash(rel), entry.Name())
			}
			source := readFileString(t, filepath.Join(dir, entry.Name()))
			if parent == "database" {
				if !strings.Contains(source, "db.memory()") {
					t.Fatalf("%s/main.leia must demonstrate the built-in database runtime", filepath.ToSlash(rel))
				}
				continue
			}
			if !strings.Contains(source, `import "github.com/never-labs/leia-`) {
				t.Fatalf("%s/main.leia must demonstrate an external Leia package import", filepath.ToSlash(rel))
			}
		}
	}
}
