package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedFunctionOmitsLegacyDocsBadge(t *testing.T) {
	for _, multipart := range []bool{false, true} {
		name := "json"
		if multipart {
			name = "multipart"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := ScaffoldFunction(dir, "public", "sample", multipart, nil); err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(dir, "functions", "public", "sample", "contracts.go"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(content), "IsPublic") {
				t.Fatal("new function still emits deprecated documentation flag")
			}
		})
	}
}
