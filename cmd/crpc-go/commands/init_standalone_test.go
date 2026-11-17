package commands

import (
	"bytes"
	"context"
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

// This explicitly exercises the public module proxy rather than silently
// replacing the generated module. Enable it in release/CI integration runs.
func TestStandaloneScaffold(t *testing.T) {
	if os.Getenv("CRPC_STANDALONE_INTEGRATION") != "1" {
		t.Skip("set CRPC_STANDALONE_INTEGRATION=1 to resolve the real published SDK and build an independent project")
	}
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	root := t.TempDir()
	cli := filepath.Join(root, "crpc-go")
	standaloneGo(t, repo, "off", "build", "-o", cli, "./cmd/crpc-go")
	project := filepath.Join(root, "standalone-api")
	command := exec.Command(cli, "init", project)
	command.Dir, command.Env = root, generatedDocsEnv(nil)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("installed CLI init: %v\n%s", err, output)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); !os.IsNotExist(err) {
		t.Fatalf("fixture parent must not be an SDK/module checkout: %v", err)
	}

	// First use the project exactly as generated: no manifest rewrites,
	// replacement, workspace, sibling checkout or fabricated module proxy.
	standaloneGo(t, project, "off", "mod", "tidy")
	var module struct {
		Path, Version, Dir string
		Replace            any
		Main               bool
	}
	if err := json.Unmarshal(standaloneGo(t, project, "off", "list", "-m", "-json", "github.com/CellsWeb/crpc-go"), &module); err != nil {
		t.Fatal(err)
	}
	if module.Path != "github.com/CellsWeb/crpc-go" || module.Version != "v1.0.0" || module.Replace != nil || module.Main {
		t.Fatalf("expected actual released SDK without replacement, got %+v", module)
	}
	t.Logf("independent published SDK: %s@%s from %s", module.Path, module.Version, module.Dir)
	mod, err := os.ReadFile(filepath.Join(project, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mod), "replace ") || strings.Contains(string(mod), "v0.0.0") || !strings.Contains(string(mod), "go 1.27.4") {
		t.Fatalf("generated module still assumes local development or stale versions:\n%s", mod)
	}
	standaloneGo(t, project, "off", "build", "./...")
	standaloneGo(t, project, "off", "test", "./...")
	for _, mode := range []string{"published", "workspace"} {
		t.Run(mode, func(t *testing.T) {
			workspace := "off"
			if mode == "workspace" {
				// The optional, explicit development workflow uses Go workspaces;
				// the generated application's go.mod remains publishable/portable.
				standaloneGo(t, root, "off", "work", "init", project, repo)
				workspace = filepath.Join(root, "go.work")
				var local struct {
					Main bool
					Dir  string
				}
				if err := json.Unmarshal(standaloneGo(t, project, workspace, "list", "-m", "-json", "github.com/CellsWeb/crpc-go"), &local); err != nil {
					t.Fatal(err)
				}
				if !local.Main || filepath.Clean(local.Dir) != repo {
					t.Fatalf("workspace did not select candidate SDK: %+v", local)
				}
			}
			binary := filepath.Join(root, "server-"+mode)
			args := []string{"build", "-o", binary}
			if os.Getenv("CRPC_SCAFFOLD_RACE") == "1" {
				args = append(args, "-race")
			}
			standaloneGo(t, project, workspace, append(args, "./cmd/server")...)
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("docs_%t", enabled), func(t *testing.T) { standaloneHTTP(t, project, binary, enabled, mode == "workspace") })
			}
		})
	}
}

func standaloneGo(t *testing.T, dir, workspace string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir, command.Env = dir, generatedDocsEnv(map[string]string{"GOWORK": workspace})
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("go %v (dir=%s GOWORK=%s): %v\n%s", args, dir, workspace, err, output)
	}
	t.Logf("go %v (GOWORK=%s) PASS", args, workspace)
	return output
}

func standaloneHTTP(t *testing.T, project, binary string, docs, candidate bool) {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:4000")
	if err != nil {
		t.Fatalf("standalone integration requires unused port 4000: %v", err)
	}
	_ = listener.Close()
	command := exec.Command(binary)
	command.Dir = project
	command.Env = generatedDocsEnv(map[string]string{"NODE_ENV": "production", "DOCS_ENABLED": fmt.Sprint(docs), "DOCS_PATH": "/reference"})
	logPath := filepath.Join(t.TempDir(), "server.log")
	logs, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout, command.Stderr = logs, logs
	if err := command.Start(); err != nil {
		_ = logs.Close()
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGTERM)
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = command.Process.Kill()
			<-done
			t.Error("server did not stop")
		}
		_ = logs.Close()
		data, _ := os.ReadFile(logPath)
		if bytes.Contains(data, []byte("DATA RACE")) {
			t.Errorf("generated server race:\n%s", data)
		}
	})
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{DisableKeepAlives: true}}
	t.Cleanup(client.CloseIdleConnections)
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get("http://127.0.0.1:4000/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		data, _ := os.ReadFile(logPath)
		t.Fatalf("server not ready:\n%s", data)
	}
	for _, uri := range []string{"/reference", "/reference/user/getProfile", "/reference/middlewares/auth", "/reference/schema.json", "/reference/metadata"} {
		status, body := docsHTTP(t, client, uri)
		if !docs {
			if status != 404 || strings.Contains(body, "Target resource identifier") || strings.Contains(body, "<html") {
				t.Fatalf("disabled docs exposed at %s: %d %s", uri, status, body)
			}
		} else if uri == "/reference" || uri == "/reference/user/getProfile" || uri == "/reference/middlewares/auth" {
			if status != 200 || !strings.Contains(body, "<html") {
				t.Fatalf("enabled docs missing at %s: %d", uri, status)
			}
			if candidate && (strings.Contains(body, ">PUBLIC</span>") || strings.Contains(body, ">PRIVATE</span>")) {
				t.Error("candidate docs badges regressed")
			}
		}
	}
	for _, tc := range []struct{ data, status, code string }{
		{`{"id":"independent"}`, "ok", ""}, {`{"id":123}`, "error", "validation_error"},
	} {
		response, err := client.Post("http://127.0.0.1:4000/rpc", "application/json", strings.NewReader(`{"namespace":"user","functionName":"getProfile","data":`+tc.data+`}`))
		if err != nil {
			t.Fatal(err)
		}
		var result struct {
			Status, Code string
			Data         struct{ ID string }
		}
		err = json.NewDecoder(response.Body).Decode(&result)
		_ = response.Body.Close()
		if err != nil || response.StatusCode != 200 || result.Status != tc.status || result.Code != tc.code {
			t.Fatalf("RPC behavior changed: HTTP%d %+v %v", response.StatusCode, result, err)
		}
		if tc.status == "ok" && result.Data.ID != "independent" {
			t.Fatalf("typed handler returned %+v", result)
		}
	}
}
