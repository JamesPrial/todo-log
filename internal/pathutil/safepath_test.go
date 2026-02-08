package pathutil_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/JamesPrial/todo-log/internal/pathutil"
)

func Test_ResolveSafePath_Cases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, baseDir string) // optional filesystem setup
		userPath  func(baseDir string) string        // produces userPath; receives baseDir
		wantErr   bool
		checkPath func(t *testing.T, baseDir, result string) // optional assertion on the returned path
	}{
		// -----------------------------------------------------------------
		// Success cases
		// -----------------------------------------------------------------
		{
			name: "relative path within base",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(baseDir, "sub"))
			},
			userPath: func(_ string) string { return "sub/file.txt" },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolvedBase, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("failed to resolve base dir: %v", err)
				}
				if !strings.HasPrefix(result, resolvedBase) {
					t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
				}
				if !strings.HasSuffix(result, "sub/file.txt") && !strings.HasSuffix(result, filepath.Join("sub", "file.txt")) {
					t.Errorf("result %q does not end with sub/file.txt", result)
				}
			},
		},
		{
			name: "absolute path within base",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(baseDir, "sub"))
			},
			userPath: func(baseDir string) string { return filepath.Join(baseDir, "sub", "file.txt") },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolvedBase, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("failed to resolve base dir: %v", err)
				}
				if !strings.HasPrefix(result, resolvedBase) {
					t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
				}
			},
		},
		{
			name: "deep nested path",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(baseDir, "a", "b", "c"))
			},
			userPath: func(_ string) string { return "a/b/c/file.txt" },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolvedBase, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("failed to resolve base dir: %v", err)
				}
				if !strings.HasPrefix(result, resolvedBase) {
					t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
				}
			},
		},
		{
			name:     "dot path resolves to base dir",
			userPath: func(_ string) string { return "." },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				// Resolve baseDir the same way the function would to compare fairly.
				resolved, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("EvalSymlinks baseDir: %v", err)
				}
				if result != resolved {
					t.Errorf("dot path resolved to %q, want %q", result, resolved)
				}
			},
		},
		{
			name: "path normalization with dot-slash and double-slash",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(baseDir, "sub"))
			},
			userPath: func(_ string) string { return "./sub//file.txt" },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolvedBase, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("failed to resolve base dir: %v", err)
				}
				if !strings.HasPrefix(result, resolvedBase) {
					t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
				}
			},
		},
		{
			name: "trailing slash on directory",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				mustMkdirAll(t, filepath.Join(baseDir, "sub"))
			},
			userPath: func(_ string) string { return "sub/" },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolvedBase, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("failed to resolve base dir: %v", err)
				}
				if !strings.HasPrefix(result, resolvedBase) {
					t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
				}
			},
		},
		{
			name: "symlink within base stays safe",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				target := filepath.Join(baseDir, "real")
				mustMkdirAll(t, target)
				link := filepath.Join(baseDir, "link")
				if err := os.Symlink(target, link); err != nil {
					t.Skipf("symlinks not supported: %v", err)
				}
			},
			userPath: func(_ string) string { return "link/file.txt" },
			wantErr:  false,
			checkPath: func(t *testing.T, baseDir, result string) {
				t.Helper()
				resolved, err := filepath.EvalSymlinks(baseDir)
				if err != nil {
					t.Fatalf("EvalSymlinks baseDir: %v", err)
				}
				if !strings.HasPrefix(result, resolved) {
					t.Errorf("result %q is not within resolved baseDir %q", result, resolved)
				}
			},
		},

		// -----------------------------------------------------------------
		// Error cases
		// -----------------------------------------------------------------
		{
			name:     "dot-dot escaping",
			userPath: func(_ string) string { return "../outside.txt" },
			wantErr:  true,
		},
		{
			name: "absolute path escaping",
			userPath: func(_ string) string {
				if runtime.GOOS == "windows" {
					return `C:\Windows\System32\config\SAM`
				}
				return "/etc/passwd"
			},
			wantErr: true,
		},
		{
			name:     "empty string",
			userPath: func(_ string) string { return "" },
			wantErr:  true,
		},
		{
			name:     "whitespace only",
			userPath: func(_ string) string { return "   " },
			wantErr:  true,
		},
		{
			name:     "null bytes in path",
			userPath: func(_ string) string { return "file\x00name.txt" },
			wantErr:  true,
		},
		{
			name: "symlink escaping base",
			setup: func(t *testing.T, baseDir string) {
				t.Helper()
				// Create a symlink inside baseDir that points outside.
				link := filepath.Join(baseDir, "escape_link")
				if err := os.Symlink(os.TempDir(), link); err != nil {
					t.Skipf("symlinks not supported: %v", err)
				}
			},
			userPath: func(_ string) string { return "escape_link/file.txt" },
			wantErr:  true,
		},
		{
			name:     "multiple dot-dot segments",
			userPath: func(_ string) string { return "../../../../../../etc/passwd" },
			wantErr:  true,
		},
		{
			name:     "dot-dot in middle of path",
			userPath: func(_ string) string { return "sub/../../outside.txt" },
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			baseDir := t.TempDir()

			if tt.setup != nil {
				tt.setup(t, baseDir)
			}

			userPath := tt.userPath(baseDir)
			result, err := pathutil.ResolveSafePath(baseDir, userPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveSafePath(%q, %q) = %q, want error", baseDir, userPath, result)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveSafePath(%q, %q) unexpected error: %v", baseDir, userPath, err)
			}

			// All successful results must be absolute paths.
			if !filepath.IsAbs(result) {
				t.Errorf("result %q is not an absolute path", result)
			}

			if tt.checkPath != nil {
				tt.checkPath(t, baseDir, result)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error message quality tests
// ---------------------------------------------------------------------------

func Test_ResolveSafePath_ErrorMessages(t *testing.T) {
	t.Parallel()

	t.Run("empty path error is descriptive", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		_, err := pathutil.ResolveSafePath(baseDir, "")
		if err == nil {
			t.Fatal("expected error for empty path")
		}
		errMsg := err.Error()
		// Error should mention the problem, not be a generic "error".
		if len(errMsg) < 5 {
			t.Errorf("error message too short to be useful: %q", errMsg)
		}
	})

	t.Run("null byte error is descriptive", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		_, err := pathutil.ResolveSafePath(baseDir, "file\x00.txt")
		if err == nil {
			t.Fatal("expected error for null byte path")
		}
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "null") && !strings.Contains(errMsg, "invalid") && !strings.Contains(errMsg, "illegal") {
			t.Errorf("error message %q does not mention null/invalid/illegal", err.Error())
		}
	})

	t.Run("escape error mentions escaping or outside", func(t *testing.T) {
		t.Parallel()
		baseDir := t.TempDir()
		_, err := pathutil.ResolveSafePath(baseDir, "../outside.txt")
		if err == nil {
			t.Fatal("expected error for escaping path")
		}
		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "escap") && !strings.Contains(errMsg, "outside") && !strings.Contains(errMsg, "traversal") && !strings.Contains(errMsg, "denied") {
			t.Errorf("error message %q does not indicate path escape", err.Error())
		}
	})
}

// ---------------------------------------------------------------------------
// Non-existent file within existing directory
// ---------------------------------------------------------------------------

func Test_ResolveSafePath_NonExistentFileInExistingDir(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	mustMkdirAll(t, filepath.Join(baseDir, "existing_dir"))

	// The file does not exist, but its parent directory does.
	result, err := pathutil.ResolveSafePath(baseDir, "existing_dir/nonexistent_file.txt")
	if err != nil {
		t.Fatalf("expected success for non-existent file in existing dir, got error: %v", err)
	}
	if !strings.HasPrefix(result, baseDir) {
		// Compare against resolved base in case of symlinks (macOS /private/var issue).
		resolvedBase, _ := filepath.EvalSymlinks(baseDir)
		if !strings.HasPrefix(result, resolvedBase) {
			t.Errorf("result %q is not within baseDir %q (resolved: %q)", result, baseDir, resolvedBase)
		}
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", path, err)
	}
}
