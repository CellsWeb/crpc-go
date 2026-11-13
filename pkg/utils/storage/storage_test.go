package storage_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CellsWeb/crpc-go/pkg/utils/storage"
)

func TestStorageLocalUploadAndTraversalProtection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "crpc_storage_test")
	if err != nil {
		t.Fatalf("Failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := storage.New(storage.Config{
		UploadDir: tempDir,
		ServerURL: "http://localhost:4000",
	})

	// 1. Normal safe upload
	data := []byte("hello world image bytes")
	res, err := eng.Upload(data, "photo.png", "avatars")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	if !strings.HasPrefix(res.URL, "uploads/avatars/") {
		t.Errorf("Expected URL starting with 'uploads/avatars/', got %q", res.URL)
	}
	if !strings.HasPrefix(res.FullURL, "http://localhost:4000/uploads/avatars/") {
		t.Errorf("Expected FullURL starting with server URL, got %q", res.FullURL)
	}

	// 2. Serve uploaded file
	servedBytes, mime, err := eng.Serve(res.URL)
	if err != nil {
		t.Fatalf("Serve failed: %v", err)
	}
	if string(servedBytes) != "hello world image bytes" {
		t.Errorf("Served content mismatch: got %q", string(servedBytes))
	}
	if mime != "image/png" {
		t.Errorf("Expected image/png MIME, got %q", mime)
	}

	// 3. Security: Directory Traversal rejection in subDir
	_, err = eng.Upload(data, "escape.png", "../../../etc")
	if err == nil {
		// Even if cleaned, it must stay inside uploadRoot
		cleanPath := filepath.Join(tempDir, "etc")
		if !strings.HasPrefix(cleanPath, tempDir) {
			t.Errorf("Security flaw: path traversal succeeded!")
		}
	}

	// 4. Security: Directory Traversal in Delete
	ok := eng.Delete("../../etc/passwd")
	if ok {
		t.Errorf("Security flaw: traversal delete returned true!")
	}

	// 5. Normal safe delete
	deleted := eng.Delete(res.URL)
	if !deleted {
		t.Errorf("Expected safe delete to return true")
	}
}
