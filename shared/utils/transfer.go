// Package utils provides shared utilities for both client and server
package utils

import (
	"crypto/sha256"
	dw "discon-wrapper"
	"encoding/hex"
	"fmt"
	"strings"
)

// GH-Cp gen: FileTransferResult holds the result of a file transfer operation
type FileTransferResult struct {
	Success     bool
	ServerPath  string
	ErrorMessage string
}

// GH-Cp gen: IsFileTransfer checks if a payload represents a file transfer
func IsFileTransfer(payload *dw.Payload) bool {
	return len(payload.FileContent) > 0 && len(payload.ServerFilePath) > 0
}

// GH-Cp gen: CreateFileTransferPayload creates a payload for file transfer
func CreateFileTransferPayload(fileContent []byte, serverFilePath string) *dw.Payload {
	return &dw.Payload{
		// Initialize required fields with empty values
		Swap:           make([]float32, 1),
		Fail:           0,
		InFile:         []byte{0},
		OutName:        []byte{0},
		Msg:            []byte{0},
		FileContent:    fileContent,
		ServerFilePath: []byte(serverFilePath + "\x00"),
	}
}

// GH-Cp gen: CreateFileTransferResponse creates a response payload for a file transfer
func CreateFileTransferResponse(success bool, message string) *dw.Payload {
	response := &dw.Payload{
		Swap:    make([]float32, 1),
		Fail:    0,
		InFile:  []byte{0},
		OutName: []byte{0},
		Msg:     make([]byte, 256), // Reserve space for message
	}

	if !success {
		response.Fail = 1
	}

	// Ensure null termination
	if !strings.HasSuffix(message, "\x00") {
		message += "\x00"
	}

	// Copy message to Msg field
	copy(response.Msg, []byte(message))

	return response
}

// GH-Cp gen: ExtractStringFromBytes extracts a null-terminated string from a byte array
func ExtractStringFromBytes(data []byte) string {
	nullIndex := strings.IndexByte(string(data), 0)
	if nullIndex >= 0 {
		return string(data[:nullIndex])
	}
	return string(data)
}

// GH-Cp gen: ComputeFileHash computes a SHA256 hash of file content
func ComputeFileHash(content []byte) string {
	hash := sha256.New()
	hash.Write(content)
	return hex.EncodeToString(hash.Sum(nil))
}

// GH-Cp gen: GetErrorMessageFromPayload extracts an error message from a payload
func GetErrorMessageFromPayload(payload *dw.Payload) string {
	if payload == nil {
		return "No payload received"
	}
	
	errMsg := string(payload.Msg)
	nullIndex := strings.IndexByte(errMsg, 0)
	if nullIndex >= 0 {
		errMsg = errMsg[:nullIndex]
	}
	return errMsg
}

// GH-Cp gen: PreparePayloadForTransmission ensures a payload is properly formatted
func PreparePayloadForTransmission(payload *dw.Payload) error {
	// Check required fields
	if payload.Swap == nil || len(payload.Swap) == 0 {
		payload.Swap = make([]float32, 1)
	}
	
	// Ensure all string fields are null-terminated
	if len(payload.InFile) == 0 || payload.InFile[len(payload.InFile)-1] != 0 {
		payload.InFile = append(payload.InFile, 0)
	}
	
	if len(payload.OutName) == 0 || payload.OutName[len(payload.OutName)-1] != 0 {
		payload.OutName = append(payload.OutName, 0)
	}
	
	if len(payload.Msg) == 0 || payload.Msg[len(payload.Msg)-1] != 0 {
		payload.Msg = append(payload.Msg, 0)
	}
	
	if len(payload.ServerFilePath) > 0 && payload.ServerFilePath[len(payload.ServerFilePath)-1] != 0 {
		payload.ServerFilePath = append(payload.ServerFilePath, 0)
	}
	
	return nil
}

// GH-Cp gen: FormatError creates a formatted error message for file transfer errors
func FormatError(operation string, err error) error {
	return fmt.Errorf("%s failed: %w", operation, err)
}