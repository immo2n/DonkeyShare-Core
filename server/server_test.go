package server

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafeJoin(testingT *testing.T) {
	tempDir, err := os.MkdirTemp("", "filelink-test")
	if err != nil {
		testingT.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	err = os.WriteFile(filepath.Join(tempDir, "file.txt"), []byte("hello"), 0644)
	if err != nil {
		testingT.Fatalf("failed to write test file: %v", err)
	}

	err = os.Mkdir(filepath.Join(tempDir, "subdir"), 0755)
	if err != nil {
		testingT.Fatalf("failed to create subdir: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, "subdir", "subfile.txt"), []byte("sub hello"), 0644)
	if err != nil {
		testingT.Fatalf("failed to write subfile: %v", err)
	}

	tests := []struct {
		rel      string
		expected string
		hasError bool
	}{
		{"", tempDir, false},
		{"file.txt", filepath.Join(tempDir, "file.txt"), false},
		{"subdir", filepath.Join(tempDir, "subdir"), false},
		{"subdir/subfile.txt", filepath.Join(tempDir, "subdir", "subfile.txt"), false},
		{"../", "", true},
		{"file.txt/../../etc/passwd", "", true},
		{"subdir/../file.txt", filepath.Join(tempDir, "file.txt"), false},
		{"/file.txt", filepath.Join(tempDir, "file.txt"), false},
	}

	for _, tc := range tests {
		testingT.Run(tc.rel, func(t *testing.T) {
			res, err := SafeJoin(tempDir, tc.rel)
			if tc.hasError {
				if err == nil {
					t.Errorf("expected error for rel %q, got nil (resolved to %q)", tc.rel, res)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for rel %q, got: %v", tc.rel, err)
				}
				cleanRes := filepath.Clean(res)
				cleanExpected := filepath.Clean(tc.expected)
				if cleanRes != cleanExpected {
					t.Errorf("expected %q for rel %q, got %q", cleanExpected, tc.rel, cleanRes)
				}
			}
		})
	}
}
