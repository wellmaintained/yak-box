// Package pathutil provides path utilities and security validation for file operations.
// This package prevents path traversal vulnerabilities (CWE-22) by implementing
// strict boundary checking on file system paths, ensuring operations remain within
// designated directories.
package pathutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathTraversal is returned when a path attempts to escape the designated boundary directory
var ErrPathTraversal = errors.New("path traversal attack detected: path attempts to escape boundary")

// ValidatePath validates that a given path is contained within a base directory
// and prevents path traversal attacks (CWE-22). The validation ensures:
// - Cleans path with filepath.Clean() to normalize
// - Converts relative paths to absolute by joining with baseDir
// - Resolves symbolic links with filepath.EvalSymlinks()
// - Verifies resolved path starts with baseDir boundary (with separator)
//
// Returns nil if path is valid, ErrPathTraversal if escaping boundary, or other errors on resolution failure.
func ValidatePath(path, baseDir string) error {
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}
	if baseDir == "" {
		return fmt.Errorf("baseDir cannot be empty")
	}

	cleanPath := filepath.Clean(path)

	absBaseDir, err := canonicalPath(baseDir)
	if err != nil {
		return fmt.Errorf("cannot resolve baseDir to absolute path: %w", err)
	}

	var absPath string
	if !filepath.IsAbs(cleanPath) {
		absPath = filepath.Join(absBaseDir, cleanPath)
	} else {
		absPath = cleanPath
	}
	resolvedPath, err := canonicalPath(absPath)
	if err != nil {
		return fmt.Errorf("cannot resolve path to absolute path: %w", err)
	}

	if resolvedPath == absBaseDir {
		return nil
	}

	if !strings.HasPrefix(resolvedPath, absBaseDir+string(filepath.Separator)) {
		return fmt.Errorf("%w: %s is not within %s", ErrPathTraversal, path, baseDir)
	}

	return nil
}

func canonicalPath(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// EvalSymlinks fails for non-existent files; in that case, resolve the
	// existing parent and then re-append the final element.
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return filepath.Clean(resolvedPath), nil
	}
	if !os.IsNotExist(err) {
		return filepath.Clean(absPath), nil
	}

	parent := filepath.Dir(absPath)
	base := filepath.Base(absPath)
	resolvedParent, parentErr := filepath.EvalSymlinks(parent)
	if parentErr != nil {
		return filepath.Clean(absPath), nil
	}
	return filepath.Clean(filepath.Join(resolvedParent, base)), nil
}
