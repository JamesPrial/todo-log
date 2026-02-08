// Package pathutil provides path resolution and validation utilities.
//
// This package implements security-focused path handling to prevent directory
// traversal attacks and ensure all paths stay within designated boundaries.
package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSafePath resolves userPath relative to baseDir, ensuring the result
// stays within baseDir after symlink resolution.
//
// This function provides defense-in-depth against path traversal attacks by:
//   - Rejecting empty or whitespace-only paths
//   - Rejecting paths containing null bytes
//   - Resolving all symlinks to their true filesystem locations
//   - Verifying the final resolved path is contained within baseDir
//
// If userPath is relative, it is joined with baseDir. If userPath is absolute,
// it is still validated to ensure it falls within baseDir after resolution.
//
// Returns an error if:
//   - userPath is empty or whitespace-only
//   - userPath contains null bytes (\x00)
//   - the resolved path escapes baseDir (including via symlinks)
//
// Example:
//
//	safePath, err := ResolveSafePath("/home/user/project", ".claude/todos.json")
//	if err != nil {
//	    // Handle path traversal attempt
//	}
//	// safePath is guaranteed to be within /home/user/project
func ResolveSafePath(baseDir, userPath string) (string, error) {
	// Reject empty or whitespace-only paths
	trimmed := strings.TrimSpace(userPath)
	if trimmed == "" {
		return "", fmt.Errorf("path is empty or whitespace-only")
	}

	// Reject paths containing null bytes
	if strings.Contains(userPath, "\x00") {
		return "", fmt.Errorf("path contains null byte")
	}

	// Build candidate path
	candidate := userPath
	if !filepath.IsAbs(userPath) {
		candidate = filepath.Join(baseDir, userPath)
	}

	// Clean the path to normalize it
	candidate = filepath.Clean(candidate)

	// Resolve symlinks in the candidate path
	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		// If the file doesn't exist yet, resolve the parent directory
		// and rejoin with the base name
		if os.IsNotExist(err) {
			parent := filepath.Dir(candidate)
			base := filepath.Base(candidate)

			resolvedParent, err := filepath.EvalSymlinks(parent)
			if err != nil {
				// Parent doesn't exist either, try resolving as much as possible
				// by continuing up the directory tree
				resolvedParent, err = resolveExistingParent(parent)
				if err != nil {
					return "", fmt.Errorf("failed to resolve parent directory: %w", err)
				}
			}

			resolved = filepath.Join(resolvedParent, base)
		} else {
			return "", fmt.Errorf("failed to resolve symlinks: %w", err)
		}
	}

	// Also resolve symlinks in baseDir
	baseResolved, err := filepath.EvalSymlinks(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	// Check containment using filepath.Rel
	// If the relative path starts with "..", it escapes baseDir
	rel, err := filepath.Rel(baseResolved, resolved)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}

	if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
		return "", fmt.Errorf("path escapes base directory: %s", userPath)
	}

	return resolved, nil
}

// resolveExistingParent walks up the directory tree until it finds an existing
// directory, then resolves symlinks for that directory and reconstructs the path.
func resolveExistingParent(path string) (string, error) {
	current := filepath.Clean(path)
	var unresolvedParts []string

	// Walk up until we find an existing directory
	for {
		if _, err := os.Stat(current); err == nil {
			// Found an existing directory
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", fmt.Errorf("failed to resolve existing parent: %w", err)
			}

			// Reconstruct the path with unresolved parts
			for i := len(unresolvedParts) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, unresolvedParts[i])
			}

			return resolved, nil
		}

		// Move up one level
		parent := filepath.Dir(current)
		if parent == current {
			// Reached the root without finding an existing directory
			return "", fmt.Errorf("no existing parent directory found")
		}

		unresolvedParts = append(unresolvedParts, filepath.Base(current))
		current = parent
	}
}
