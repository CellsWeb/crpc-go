package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The scaffold currently listens on port 4000. Keep these subprocess cases
// serial and fail clearly if another service owns that port; never stop it.
func TestGeneratedServerDocsEnvironment(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:4000")
	if err != nil {
		t.Fatalf("generated-server integration requires unused port 4000: %v", err)
	}
	_ = listener.Close()
	project, binary := buildGeneratedDocsServer(t)
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	t.Cleanup(client.CloseIdleConnections)
	cases := []struct {
		name       string
		env        map[string]string
		envFile    string
		sourceFile bool
		afterFile  string
		enabled    bool
		path       string
		invalid    bool
	}{
		{name: "development default", enabled: true},
		{name: "production default", env: map[string]string{"NODE_ENV": "production"}},
		{name: "development explicit false", env: map[string]string{"DOCS_ENABLED": "false"}},
		{name: "production explicit true", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "true"}, enabled: true},
		{name: "production explicit false", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "false"}},
		{name: "custom path", env: map[string]string{"DOCS_PATH": "/api/reference"}, enabled: true, path: "/api/reference"},
		{name: "production custom path", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "true", "DOCS_PATH": "/api/reference"}, enabled: true, path: "/api/reference"},
		{name: "mixed case true", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "TrUe"}, enabled: true},
		{name: "mixed case false", env: map[string]string{"DOCS_ENABLED": "FaLsE"}},
		{name: "padded true and path", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": " \tTRUE\n", "DOCS_PATH": " \t/reference\t "}, enabled: true, path: "/reference"},
		{name: "padded false", env: map[string]string{"DOCS_ENABLED": " \tFaLsE\n"}},
		{name: "numeric true", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "1"}, enabled: true},
		{name: "numeric false", env: map[string]string{"DOCS_ENABLED": "0"}},
		{name: "Go boolean t alias", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "t"}, enabled: true},
		{name: "Go boolean F alias", env: map[string]string{"DOCS_ENABLED": "F"}},
		{name: "blank development uses default", env: map[string]string{"DOCS_ENABLED": " \t", "DOCS_PATH": " \t"}, enabled: true},
		{name: "empty production uses default", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": ""}},
		{name: "blank production uses default", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": " \t"}},
		{name: "dotenv alone is not loaded", env: map[string]string{"NODE_ENV": "production"}, envFile: "NODE_ENV=development\nDOCS_ENABLED=true\nDOCS_PATH=/from-file\n"},
		{name: "explicitly source dotenv enable", envFile: "NODE_ENV=production\nDOCS_ENABLED=true\nDOCS_PATH=/from-file\n", sourceFile: true, enabled: true, path: "/from-file"},
		{name: "explicitly source dotenv disable", envFile: "NODE_ENV=development\nDOCS_ENABLED=false\nDOCS_PATH=/from-file\n", sourceFile: true, path: "/from-file"},
		{name: "shell file overrides earlier exports", env: map[string]string{"DOCS_ENABLED": "true"}, envFile: "NODE_ENV=development\nDOCS_ENABLED=false\n", sourceFile: true},
		{name: "later export overrides sourced file", envFile: "NODE_ENV=production\nDOCS_ENABLED=true\nDOCS_PATH=/from-file\n", sourceFile: true, afterFile: "export DOCS_ENABLED=false; ", path: "/from-file"},
		{name: "invalid production yes", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "yes"}, invalid: true},
		{name: "invalid production number", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "2"}, invalid: true},
		{name: "invalid production typo", env: map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": "tru"}, invalid: true},
		{name: "invalid development", env: map[string]string{"DOCS_ENABLED": "yes"}, invalid: true},
		{name: "invalid sourced production", envFile: "NODE_ENV=production\nDOCS_ENABLED=invalid\n", sourceFile: true, invalid: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			writeFixture(t, filepath.Join(project, ".env"), tc.envFile)
			cmd := exec.Command(binary)
			if tc.sourceFile {
				cmd = exec.Command("sh", "-c", `set -a; . ./.env; set +a; `+tc.afterFile+`exec "$@"`, "crpc010-env", binary)
			}
			cmd.Dir = project
			cmd.Env = generatedDocsEnv(tc.env)
			logPath := filepath.Join(t.TempDir(), "server.log")
			logFile, err := os.Create(logPath)
			if err != nil {
				t.Fatal(err)
			}
			cmd.Stdout, cmd.Stderr = logFile, logFile
			if err := cmd.Start(); err != nil {
				_ = logFile.Close()
				t.Fatal(err)
			}
			exited := make(chan struct{})
			go func() { _ = cmd.Wait(); close(exited) }()
			t.Cleanup(func() {
				_ = cmd.Process.Signal(syscall.SIGTERM)
				select {
				case <-exited:
				case <-time.After(5 * time.Second):
					_ = cmd.Process.Kill()
					<-exited
					t.Error("generated server did not shut down after SIGTERM")
				}
				_ = logFile.Close()
				logs, _ := os.ReadFile(logPath)
				if strings.Contains(string(logs), "DATA RACE") {
					t.Errorf("race in generated server:\n%s", logs)
				}
			})
			ready := false
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) {
				select {
				case <-exited:
					logs, _ := os.ReadFile(logPath)
					if tc.invalid {
						if cmd.ProcessState.ExitCode() == 0 || !strings.Contains(string(logs), "DOCS_ENABLED") {
							t.Fatalf("expected nonzero DOCS_ENABLED configuration error: %s", logs)
						}
						return
					}
					t.Fatalf("generated server exited before HTTP readiness: %s", logs)
				default:
				}
				res, err := client.Get("http://127.0.0.1:4000/health")
				if err == nil {
					_, _ = io.Copy(io.Discard, res.Body)
					_ = res.Body.Close()
					ready = res.StatusCode == 200
					if ready {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
			}
			if !ready {
				t.Fatal("generated server did not become ready")
			}
			if tc.invalid {
				t.Fatal("invalid DOCS_ENABLED accepted: generated server started instead of rejecting configuration")
			}
			endpoint := tc.path
			if endpoint == "" {
				endpoint = "/docs"
			}
			for _, suffix := range []string{"", "/", "?probe=1", "/user/getProfile", "/fn/user.getProfile", "/middlewares/auth"} {
				status, body := docsHTTP(t, client, endpoint+suffix)
				expected := 404
				if tc.enabled {
					expected = 200
				}
				if status != expected {
					t.Fatalf("GET %s: got %d, want %d (DOCS_ENABLED=%q DOCS_PATH=%q)", endpoint+suffix, status, expected, tc.env["DOCS_ENABLED"], tc.env["DOCS_PATH"])
				}
				if !tc.enabled {
					for _, marker := range []string{"<html", "Handler for user", "Target resource identifier", "sidebar-logo-badge"} {
						if strings.Contains(body, marker) {
							t.Errorf("disabled docs exposed %q at %s", marker, endpoint+suffix)
						}
					}
				} else if suffix == "/user/getProfile" {
					for _, marker := range []string{"Target resource identifier", "Handler for user.getProfile"} {
						if !strings.Contains(body, marker) {
							t.Errorf("enabled false-badge function docs missing %q", marker)
						}
					}
				}
				if tc.enabled && (strings.Contains(body, ">PUBLIC</span>") || strings.Contains(body, ">PRIVATE</span>")) {
					t.Error("legacy badge still rendered")
				}
			}
			if endpoint != "/docs" || !tc.enabled {
				if status, _ := docsHTTP(t, client, "/docs"); status != 404 {
					t.Errorf("old/disabled /docs remains accessible: %d", status)
				}
			}
			if !tc.enabled {
				for _, uri := range []string{endpoint + "/schema.json", endpoint + "/metadata", endpoint + "/search", "/openapi.json", "/schemas"} {
					if status, _ := docsHTTP(t, client, uri); status != 404 {
						t.Errorf("disabled documentation endpoint %s returned %d", uri, status)
					}
				}
			}
			for _, authorized := range []bool{false, true} {
				request, err := http.NewRequest("POST", "http://127.0.0.1:4000/rpc", bytes.NewBufferString(`{"namespace":"user","functionName":"getProfile","data":{"id":"fixture"}}`))
				if err != nil {
					t.Fatal(err)
				}
				request.Header.Set("Content-Type", "application/json")
				if authorized {
					request.Header.Set("Authorization", "Bearer valid")
				}
				response, err := client.Do(request)
				if err != nil {
					t.Fatal(err)
				}
				var result struct {
					Status, Code string
					Data         struct{ ID string }
				}
				err = json.NewDecoder(response.Body).Decode(&result)
				_ = response.Body.Close()
				if err != nil || response.StatusCode != 200 {
					t.Fatalf("RPC envelope/status changed: %v, status %d", err, response.StatusCode)
				}
				if authorized {
					if result.Status != "ok" || result.Data.ID != "fixture" {
						t.Errorf("authorized false-badge typed handler failed: %+v", result)
					}
				} else if result.Status != "error" || result.Code != "unauthorized" {
					t.Errorf("middleware authorization changed: %+v", result)
				}
			}
		})
	}
}

func buildGeneratedDocsServer(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	project := filepath.Join(t.TempDir(), "app")
	if err := InitProject(project); err != nil {
		t.Fatal(err)
	}
	// Resolve only the local SDK and already pinned dependencies. Generated main
	// stays byte-for-byte intact; no alternate entrypoint or parser substitute.
	mod, err := os.ReadFile(filepath.Join(repo, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	localMod := strings.Replace(string(mod), "module github.com/CellsWeb/crpc-go", "module app", 1)
	localMod += fmt.Sprintf("\nrequire github.com/CellsWeb/crpc-go v0.0.0\nreplace github.com/CellsWeb/crpc-go => %s\n", filepath.ToSlash(repo))
	writeFixture(t, filepath.Join(project, "go.mod"), localMod)
	sum, err := os.ReadFile(filepath.Join(repo, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Join(project, "go.sum"), string(sum))
	// Developer-customizable test fixtures: exercise real middleware enforcement
	// and ignored compatibility metadata while retaining generated main, typed handler and schema.
	writeFixture(t, filepath.Join(project, "middlewares/auth/zz_crpc010_fixture.go"), `package auth
import "github.com/CellsWeb/crpc-go/pkg/rpc"
func init() {
 rpc.RegisterMiddleware("auth", "Regression auth fixture", func(ctx *rpc.Context, headers map[string]string, options map[string]any) error {
  if headers["authorization"] != "Bearer valid" { return rpc.NewError(rpc.ErrUnauthorized, "Denied") }
  return nil
 }, OptionsSchema, HeadersSchema)
}
`)
	contractsPath := filepath.Join(project, "functions/user/getProfile/contracts.go")
	contracts, err := os.ReadFile(contractsPath)
	if err != nil {
		t.Fatal(err)
	}
	writeFixture(t, contractsPath, strings.Replace(string(contracts), "var Metadata = rpc.FunctionMetadata{", "var Metadata = rpc.FunctionMetadata{\nIsPublic: false,", 1))
	build := func(args ...string) {
		t.Helper()
		cmd := exec.Command("go", args...)
		cmd.Dir, cmd.Env = project, generatedDocsEnv(nil)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("generated project go %v failed: %v\n%s", args, err, output)
		}
	}
	build("build", "-mod=mod", "./...")
	binary := filepath.Join(project, "server")
	args := []string{"build", "-mod=mod", "-o", binary}
	if os.Getenv("CRPC_SCAFFOLD_RACE") == "1" {
		args = append(args, "-race")
	}
	build(append(args, "./cmd/server")...)
	return project, binary
}

func generatedDocsEnv(values map[string]string) []string {
	var env []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "NODE_ENV" && key != "DOCS_ENABLED" && key != "DOCS_PATH" && key != "GOWORK" && key != "GORACE" {
			env = append(env, entry)
		}
	}
	env = append(env, "GOWORK=off", "GORACE=atexit_sleep_ms=0")
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func writeFixture(t *testing.T, file, content string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func docsHTTP(t *testing.T, client *http.Client, uri string) (int, string) {
	t.Helper()
	response, err := client.Get("http://127.0.0.1:4000" + uri)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, string(body)
}
