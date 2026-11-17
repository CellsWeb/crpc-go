package commands

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"
)

func TestScaffoldModuleConfiguration(t *testing.T) {
	project := filepath.Join(t.TempDir(), "independent")
	if err := InitProject(project); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "mod", "edit", "-json")
	command.Dir, command.Env = project, generatedDocsEnv(nil)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("invalid generated module: %v\n%s", err, output)
	}
	var module struct {
		Go      string
		Module  struct{ Path string }
		Require []struct{ Path, Version string }
		Replace []any
	}
	if err := json.Unmarshal(output, &module); err != nil {
		t.Fatal(err)
	}
	if module.Module.Path != "independent" || module.Go != "1.27.4" || len(module.Replace) != 0 || len(module.Require) != 1 || module.Require[0].Path != "github.com/CellsWeb/crpc-go" || module.Require[0].Version != releasedSDKVersion {
		t.Fatalf("unexpected standalone module: %s", output)
	}
	// Init must not create an implicit workspace that redirects sibling projects.
	if _, err := os.Stat(filepath.Join(project, "go.work")); !os.IsNotExist(err) {
		t.Fatalf("implicit workspace created: %v", err)
	}
}

func TestScaffoldSDKVersion(t *testing.T) {
	// Version strings here are unit fixtures only; no synthetic tags are fetched.
	for _, tc := range []struct {
		name, path, version string
		replacement         bool
		want                string
	}{
		{"source", "github.com/CellsWeb/crpc-go", "(devel)", false, releasedSDKVersion},
		{"missing_version", "github.com/CellsWeb/crpc-go", "", false, releasedSDKVersion},
		{"released", "github.com/CellsWeb/crpc-go", "v1.0.0", false, "v1.0.0"},
		{"versioned_prerelease", "github.com/CellsWeb/crpc-go", "v1.2.3-rc.1", false, "v1.2.3-rc.1"},
		{"versioned_pseudo", "github.com/CellsWeb/crpc-go", "v1.0.1-0.20261117000000-abcdef123456", false, "v1.0.1-0.20261117000000-abcdef123456"},
		{"wrapper", "example.com/other-cli", "v9.0.0", false, releasedSDKVersion},
		{"replaced", "github.com/CellsWeb/crpc-go", "v1.2.3", true, releasedSDKVersion},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Path: tc.path, Version: tc.version}}
			if tc.replacement {
				info.Main.Replace = &debug.Module{Path: "/local/sdk"}
			}
			if got := scaffoldSDKVersion(info); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
	if got := scaffoldSDKVersion(nil); got != releasedSDKVersion {
		t.Fatalf("missing build info selected %s", got)
	}
	for _, key := range []string{"vcs", "vcs.revision", "vcs.modified", "vcs.time"} {
		t.Run("local_"+key, func(t *testing.T) {
			info := &debug.BuildInfo{Main: debug.Module{Path: "github.com/CellsWeb/crpc-go", Version: "v1.0.1-0.20261117000000-abcdef123456"}, Settings: []debug.BuildSetting{{Key: key, Value: "fixture"}}}
			if got := scaffoldSDKVersion(info); got != releasedSDKVersion {
				t.Fatalf("unpublished VCS build selected %s", got)
			}
		})
	}
}
