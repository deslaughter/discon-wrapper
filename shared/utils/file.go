// Package utils provides shared utilities for both client and server
package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// GH-Cp gen: FileExists checks if a file exists and is not a directory
func FileExists(filename string) bool {
	// GH-Cp gen: Added nil/empty check to prevent segmentation fault
	if filename == "" {
		return false
	}

	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// GH-Cp gen: ReadFileContents reads the entire content of a file
func ReadFileContents(filePath string) ([]byte, error) {
	if !FileExists(filePath) {
		return nil, fmt.Errorf("file does not exist: %s", filePath)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	return content, nil
}

// GH-Cp gen: ValidateFileName checks if a filename is safe and doesn't contain suspicious patterns
func ValidateFileName(filename string) error {
	// Remove null terminators
	filename = strings.ReplaceAll(filename, "\x00", "")

	// Check if filename contains suspicious patterns
	suspiciousPatterns := []string{"../", "/..", "~", "$", "|", ";", "&", "\\"}
	for _, pattern := range suspiciousPatterns {
		if strings.Contains(filename, pattern) {
			return fmt.Errorf("filename contains invalid pattern: %s", pattern)
		}
	}

	// Ensure filename is just a basename, not a path
	if filepath.Base(filename) != filename {
		return fmt.Errorf("filename must not contain path separators")
	}

	return nil
}

// GH-Cp gen: GenerateServerFilePath creates a unique server-side path for a file
func GenerateServerFilePath(content []byte, originalFilename string) string {
	// Create a unique filename using SHA-256 hash of content and original filename
	hash := sha256.New()
	hash.Write(content)
	hash.Write([]byte(filepath.Base(originalFilename)))
	hashString := hex.EncodeToString(hash.Sum(nil))

	// Generate a server path - use just the filename with a unique prefix
	return fmt.Sprintf("input_%s_%s", hashString[:8], filepath.Base(originalFilename))
}

// GH-Cp gen: SafeTrimString removes null terminators from the end of a string
func SafeTrimString(s string) string {
	return strings.TrimRight(s, "\x00")
}

// GH-Cp gen: CreateTempFile creates a temp file with a unique name based on original path and connID
func CreateTempFile(originalPath string, connID int32) (string, error) {
	outFile, err := os.CreateTemp(".", fmt.Sprintf("%s-%03d-", filepath.Base(originalPath), connID))
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	inFile, err := os.Open(originalPath)
	if err != nil {
		return "", err
	}
	defer inFile.Close()

	// Copy the file contents
	_, err = io.Copy(outFile, inFile)
	if err != nil {
		return "", err
	}

	return outFile.Name(), nil
}