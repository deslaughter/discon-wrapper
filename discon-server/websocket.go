package main

// #include <stdlib.h>
// void discon(int connID, float* avrSWAP, int* aviFAIL, char* accINFILE, char* avcOUTNAME, char* avcMSG);
// int load_shared_library(int connID, const char* library_path, const char* function_name);
// void unload_shared_library(int connID);
import "C"
import (
	dw "discon-wrapper"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
	"unsafe"

	// GH-Cp gen: Use the shared utilities package
	"discon-wrapper/shared/utils"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{}

// Unique identifier for the websocket connection
var connectionID int32

// Mutex to protect the connectionID variable
var connectionIDMutex sync.Mutex

// WaitGroup to wait for all goroutines to finish
var wg sync.WaitGroup

// GH-Cp gen: Map of temporary files created for each connection
var tempFiles = make(map[int32][]string)
var tempFilesMutex sync.Mutex

// GH-Cp gen: Handle file transfer from client to server - updated to use shared utilities
func handleFileTransfer(connID int32, payload *dw.Payload, logger *utils.DebugLogger) (*dw.Payload, error) {
	// Get server file path from payload
	serverFilePath := utils.ExtractStringFromBytes(payload.ServerFilePath)

	// Validate filename for security
	err := utils.ValidateFileName(serverFilePath)
	if err != nil {
		errMsg := fmt.Sprintf("Security error: %v", err)
		response := utils.CreateFileTransferResponse(false, errMsg)
		return response, fmt.Errorf("file validation error: %w", err)
	}

	// Verify file contents with hash
	contentHash := utils.ComputeFileHash(payload.FileContent)
	
	logger.Debug("Received file transfer request for %s (size: %d bytes, hash: %s)",
		serverFilePath, len(payload.FileContent), contentHash[:8])

	// Create the file with the server file path
	file, err := os.Create(serverFilePath)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to create file: %v", err)
		response := utils.CreateFileTransferResponse(false, errMsg)
		return response, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write the file contents
	_, err = file.Write(payload.FileContent)
	if err != nil {
		errMsg := fmt.Sprintf("Failed to write file: %v", err)
		response := utils.CreateFileTransferResponse(false, errMsg)
		os.Remove(serverFilePath) // Clean up the partial file
		return response, fmt.Errorf("failed to write file: %w", err)
	}

	// Register the file to be cleaned up when the connection closes
	tempFilesMutex.Lock()
	tempFiles[connID] = append(tempFiles[connID], serverFilePath)
	tempFilesMutex.Unlock()

	logger.Debug("File %s created successfully", serverFilePath)

	// Create success response
	successMsg := fmt.Sprintf("File transferred successfully: %s", serverFilePath)
	return utils.CreateFileTransferResponse(true, successMsg), nil
}

func ServeWs(w http.ResponseWriter, r *http.Request, debugLevel int) {
	wg.Add(1)
	defer wg.Done()

	// Get unique identifier for this connection
	connectionIDMutex.Lock()
	connID := connectionID
	connectionID++
	if connectionID > 8191 {
		connectionID = 0
	}
	connectionIDMutex.Unlock()

	// GH-Cp gen: Create a connection-specific logger with the connection ID
	logger := utils.NewConnectionLogger(debugLevel, "discon-server", connID)

	// Read controller path and function name from post parameters
	params, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		http.Error(w, "Error parsing url parameters: "+err.Error(), http.StatusInternalServerError)
		return
	}
	path := params.Get("path")
	proc := params.Get("proc")

	logger.Debug("Received request to load function '%s' from shared controller '%s'", proc, path)

	// Check if controller exists at path
	if !utils.FileExists(path) {
		http.Error(w, "Controller not found at '"+path+"'", http.StatusInternalServerError)
		return
	}

	// Create a copy of the shared library with a unique suffix
	tmpPath, err := utils.CreateTempFile(path, connID)
	if err != nil {
		http.Error(w, "Error duplicating controller: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(tmpPath)

	logger.Debug("Duplicated controller to '%s'", tmpPath)

	// GH-Cp gen: Initialize tempFiles entry for this connection
	tempFilesMutex.Lock()
	tempFiles[connID] = make([]string, 0)
	tempFilesMutex.Unlock()

	// GH-Cp gen: Clean up any temporary files when connection closes
	defer func() {
		tempFilesMutex.Lock()
		fileList := tempFiles[connID]
		delete(tempFiles, connID)
		tempFilesMutex.Unlock()

		// Remove all temporary files for this connection
		for _, filePath := range fileList {
			logger.Debug("Cleaning up temporary file: %s", filePath)
			os.Remove(filePath)
		}
	}()

	// Load the shared library
	libraryPath := C.CString(tmpPath)
	defer C.free(unsafe.Pointer(libraryPath))
	functionName := C.CString(proc)
	defer C.free(unsafe.Pointer(functionName))
	status := C.load_shared_library(C.int(connID), libraryPath, functionName)
	if status == 1 {
		http.Error(w, "Error loading shared library", http.StatusInternalServerError)
		return
	} else if status == 2 {
		http.Error(w, "Error loading function from shared library", http.StatusInternalServerError)
		return
	}

	logger.Debug("Library and function loaded successfully")

	// Convert connection to a websocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()

	// Log client connection info 
	logger.Debug("New WebSocket connection established from %s", ws.RemoteAddr().String())

	// Create payload structure
	payload := dw.Payload{}

	// Loop while receiving messages over socket
	for {
		// If not in debug mode, set read deadline to 5 seconds
		// This will disconnect the client if no message is received in 5 seconds
		// which allows the controller to be unloaded and deleted
		if debugLevel == 0 {
			ws.SetReadDeadline(time.Now().Add(time.Second * 5))
		}

		// Read message from websocket
		messageType, b, err := ws.ReadMessage()
		if err != nil {
			logger.Debug("WebSocket read error: %v", err)
			break
		}

		if messageType != websocket.BinaryMessage {
			logger.Debug("Received non-binary message type: %d", messageType)
			continue
		}

		err = payload.UnmarshalBinary(b)
		if err != nil {
			logger.Error("Failed to unmarshal payload: %v", err)
			break
		}

		// GH-Cp gen: Log received payload using the logger
		logger.Verbose("received payload: %v", payload)

		// GH-Cp gen: Check if the payload is a file transfer using shared utility
		if utils.IsFileTransfer(&payload) {
			response, err := handleFileTransfer(connID, &payload, logger)
			if err != nil {
				logger.Error("handleFileTransfer: %v", err)
			}
			b, err = response.MarshalBinary()
			if err != nil {
				logger.Error("Failed to marshal response: %v", err)
				break
			}
			err = ws.WriteMessage(websocket.BinaryMessage, b)
			if err != nil {
				logger.Error("Failed to write response: %v", err)
				break
			}
			continue
		}

		// Call the function from the shared library with data in payload
		C.discon(C.int(connID),
			(*C.float)(unsafe.Pointer(&payload.Swap[0])),
			(*C.int)(unsafe.Pointer(&payload.Fail)),
			(*C.char)(unsafe.Pointer(&payload.InFile[0])),
			(*C.char)(unsafe.Pointer(&payload.OutName[0])),
			(*C.char)(unsafe.Pointer(&payload.Msg[0])))

		// Convert payload to binary and send over websocket
		b, err = payload.MarshalBinary()
		if err != nil {
			logger.Error("Failed to marshal payload: %v", err)
			break
		}
		err = ws.WriteMessage(websocket.BinaryMessage, b)
		if err != nil {
			logger.Error("Failed to write message: %v", err)
			break
		}

		// GH-Cp gen: Log sent payload using the logger
		logger.Verbose("sent payload: %v", payload)
	}

	logger.Debug("WebSocket connection closed")
	
	// Unload the shared library
	C.unload_shared_library(C.int(connID))
}
