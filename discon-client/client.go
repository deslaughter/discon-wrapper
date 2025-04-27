package main

import "C"

import (
	"crypto/tls"
	dw "discon-wrapper"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	// GH-Cp gen: Import shared utilities
	"discon-wrapper/shared/utils"

	"github.com/gorilla/websocket"
)

const program = "discon-client"
const version = "v0.2.0"

var debugLevel int = 0

var ws *websocket.Conn
var payload dw.Payload
var sentSwapFile *os.File
var recvSwapFile *os.File

// GH-Cp gen: Map to store server-side file paths for transferred files
var serverFilePaths = make(map[string]string)

// Added map to track if a file is the primary input file
var isPrimaryInputFile = make(map[string]bool)

// Added map for storing file content replacements
var fileContentReplacements = make(map[string][]struct {
	Original string
	Replaced string
})

// Track if additional files have been processed
var additionalFilesProcessed bool = false

// GH-Cp gen: Logger for client operations
var logger *utils.DebugLogger

// Process the DISCON_ADDITIONAL_FILES environment variable
func processAdditionalFiles() error {
	additionalFilesStr, found := os.LookupEnv("DISCON_ADDITIONAL_FILES")
	if (!found || additionalFilesStr == "") {
		logger.Debug("No DISCON_ADDITIONAL_FILES specified")
		return nil
	}

	// Split the semicolon-separated list
	additionalFiles := strings.Split(additionalFilesStr, ";")
	logger.Debug("Processing %d additional files", len(additionalFiles))

	// Process all additional files
	for _, filePath := range additionalFiles {
		filePath = strings.TrimSpace(filePath)
		if filePath == "" {
			continue
		}

		if !utils.FileExists(filePath) {
			return fmt.Errorf("additional file does not exist: %s", filePath)
		}

		// Send the file but don't track errors - we'll collect and report them later
		serverPath, err := sendFileToServer(filePath)
		if err != nil {
			return fmt.Errorf("failed to send additional file %s: %w", filePath, err)
		}

		logger.Debug("Additional file %s transferred to server at %s", filePath, serverPath)
	}

	return nil
}

// Function to update file references in a content buffer
func updateFileReferences(content []byte) []byte {
	contentStr := string(content)
	
	// Go through each file that might need replacement
	for localPath, serverPath := range serverFilePaths {
		// Skip the primary input file itself
		if isPrimaryInputFile[localPath] {
			continue
		}
		
		// Also try to replace just the filename (in case only the filename is referenced)
		// but only if it's not already a server path (doesn't start with "input_")
		localFilename := filepath.Base(localPath)
		serverFilename := filepath.Base(serverPath)
		
		// Only replace the filename if it's not already a server path (doesn't start with "input_")
		if !strings.HasPrefix(localFilename, "input_") {
			// Replace only whole words to avoid partial replacements within other words
			contentStr = strings.ReplaceAll(contentStr, localFilename, serverFilename)
		}

		logger.Verbose("Replaced references from %s to %s in input file", localFilename, serverFilename)
	}
	
	return []byte(contentStr)
}

// GH-Cp gen: Function to send file to server and get server path - refactored to use shared utilities
func sendFileToServer(filePath string) (string, error) {
	// Check if we've already sent this file
	if serverPath, exists := serverFilePaths[filePath]; exists {
		return serverPath, nil
	}

	// Read the file contents
	content, err := utils.ReadFileContents(filePath)
	if err != nil {
		return "", err
	}
	
	// If this is the primary input file and we have additional files transferred,
	// update references to those files in the content
	if isPrimaryInputFile[filePath] && len(serverFilePaths) > 0 {
		logger.Debug("Updating file references in content for %s", filePath)
		content = updateFileReferences(content)
	}

	// Generate a server path using shared utility
	serverPath := utils.GenerateServerFilePath(content, filePath)

	// Create a file transfer payload using shared utility
	fileTransferPayload := utils.CreateFileTransferPayload(content, serverPath)

	logger.Debug("Sending file %s to server (size: %d bytes)", filePath, len(content))

	// Send the file transfer payload to the server
	b, err := fileTransferPayload.MarshalBinary()
	if err != nil {
		return "", utils.FormatError("marshaling file transfer payload", err)
	}

	err = ws.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return "", utils.FormatError("sending file to server", err)
	}

	// Wait for server response
	_, resp, err := ws.ReadMessage()
	if err != nil {
		return "", utils.FormatError("receiving server response", err)
	}

	// Unmarshal the response
	var responsePayload dw.Payload
	err = responsePayload.UnmarshalBinary(resp)
	if err != nil {
		return "", utils.FormatError("unmarshaling server response", err)
	}

	// Check if the file transfer succeeded
	if responsePayload.Fail != 0 {
		// Get the error message from the Msg field using shared utility
		errMsg := utils.GetErrorMessageFromPayload(&responsePayload)
		return "", fmt.Errorf("file transfer failed: %s", errMsg)
	}

	// Store the server path for future use
	serverFilePaths[filePath] = serverPath

	logger.Debug("File %s transferred successfully to server at %s", filePath, serverPath)

	return serverPath, nil
}

func init() {
	// Print info
	fmt.Println("Loaded", program, version)

	// Get debug flag from environment variable
	csvFileName := "discon_swap"
	debugStr, debug := os.LookupEnv("DISCON_CLIENT_DEBUG")
	if debug {
		var err error
		debugLevel, err = strconv.Atoi(debugStr)
		if err != nil {
			// If not a number, treat as filename and set debug level to 1
			csvFileName = debugStr
			debugLevel = 1
		}

		// Only create log files if debugLevel > 0
		if debugLevel > 0 {
			sentSwapFile, err = os.Create(csvFileName + "_sent.csv")
			if err != nil {
				log.Fatal("discon-client: error creating sent swap file:", err)
			}
			recvSwapFile, err = os.Create(csvFileName + "_recv.csv")
			if err != nil {
				log.Fatal("discon-client: error creating recv swap file:", err)
			}
		}
	}

	// GH-Cp gen: Initialize the logger
	logger = utils.NewDebugLogger(debugLevel, "discon-client")

	// Get discon-server address from environment variable
	serverAddr, found := os.LookupEnv("DISCON_SERVER_ADDR")
	if !found {
		log.Fatal("discon-client: environment variable DISCON_SERVER_ADDR not set (e.g. 'localhost:8080' or 'https://controller.domain.com')")
	}

	// Get shared library path from environment variable
	disconPath, found := os.LookupEnv("DISCON_LIB_PATH")
	if !found {
		log.Fatal("discon-client: environment variable DISCON_LIB_PATH not set (e.g. 'discon.dll')")
	}

	// Get shared library function name from environment variable
	disconFunc, found := os.LookupEnv("DISCON_LIB_PROC")
	if !found {
		log.Fatal("discon-client: environment variable DISCON_LIB_PROC not set (e.g. 'discon')")
	}

	logger.Debug("DISCON_SERVER_ADDR= %s", serverAddr)
	logger.Debug("DISCON_LIB_PATH= %s", disconPath)
	logger.Debug("DISCON_LIB_PROC= %s", disconFunc)
	logger.Debug("DISCON_CLIENT_DEBUG= %d", debugLevel)
	logger.Debug("DISCON_ADDITIONAL_FILES= %s", os.Getenv("DISCON_ADDITIONAL_FILES"))

	// Determine if we're using HTTPS/WSS based on the provided server address
	var wsURL string
	if strings.HasPrefix(strings.ToLower(serverAddr), "http://") {
		// HTTP URL provided - use ws://
		serverAddr = strings.TrimPrefix(serverAddr, "http://")
		wsURL = fmt.Sprintf("ws://%s/ws", serverAddr)
	} else if strings.HasPrefix(strings.ToLower(serverAddr), "https://") {
		// HTTPS URL provided - use wss://
		serverAddr = strings.TrimPrefix(serverAddr, "https://")
		wsURL = fmt.Sprintf("wss://%s/ws", serverAddr)
	} else {
		// No protocol provided, assume ws:// (non-secure)
		wsURL = fmt.Sprintf("ws://%s/ws", serverAddr)
	}

	// Create a URL object
	u, err := url.Parse(wsURL)
	if err != nil {
		log.Fatal(err)
	}

	// Add query parameters for shared library path and proc
	u.RawQuery = url.Values{"path": {disconPath}, "proc": {disconFunc}}.Encode()

	logger.Debug("connecting to discon-server at '%s'", u.String())

	// Connect to websocket server
	dialer := websocket.DefaultDialer
	// If using wss (secure WebSocket), we might need to skip certificate verification in some cases
	if strings.HasPrefix(u.String(), "wss://") {
		dialer.TLSClientConfig = &tls.Config{
			// For production, you should properly handle certificates
			// InsecureSkipVerify: true, // Uncomment this line to skip certificate verification (not recommended for production)
		}
	}

	ws, _, err = dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("discon-client: error connecting to discon-server at %s: %s", serverAddr, err)
	}

	logger.Debug("Connected to discon-server at '%s'", u.String())
}

//export DISCON
func DISCON(avrSwap *C.float, aviFail *C.int, accInFile, avcOutName, avcMsg *C.char) {
	// Get first 130 entries of swap array
	swap := (*[1 << 24]float32)(unsafe.Pointer(avrSwap))

	// Get array sizes
	swapSize := int(swap[128])   // Maximum size of swap array
	inFileSize := int(swap[49])  // Maximum size of inFile string
	outNameSize := int(swap[63]) // Maximum size of outName string
	msgSize := int(swap[48])     // Maximum size of msg string

	// Resize payload arrays to match sizes
	if len(payload.Swap) != swapSize {
		payload.Swap = make([]float32, swapSize)
	}

	logger.Verbose("size of avrSWAP:    % 5d", swapSize)
	logger.Verbose("size of accINFILE:  % 5d", inFileSize)
	logger.Verbose("size of avcOUTNAME: % 5d", outNameSize)
	logger.Verbose("size of avcMSG:     % 5d", msgSize)

	// GH-Cp gen: Get the input file path from accInFile with safer handling
	var inFilePath string
	if accInFile != nil && inFileSize > 0 {
		// Safely get the file path - ensure we don't read past the end of the array
		safeSize := inFileSize
		if safeSize > 1 {
			safeSize-- // -1 to exclude null terminator if present
		}
		inFilePath = string((*[1 << 24]byte)(unsafe.Pointer(accInFile))[:safeSize])
		// Remove any null terminators from the end of the string
		inFilePath = utils.SafeTrimString(inFilePath) 
	}
	
	// Process additional files before handling the main input file (but only once)
	if !additionalFilesProcessed {
		// Process any additional files specified via environment variable
		if err := processAdditionalFiles(); err != nil {
			logger.Error("Error processing additional files: %v", err)
			// Set failure flag
			*aviFail = C.int(1)
			// Set error message
			errMsg := fmt.Sprintf("Additional files transfer failed: %v", err)
			copy((*[1 << 24]byte)(unsafe.Pointer(avcMsg))[:msgSize], []byte(errMsg))
			return
		}
		additionalFilesProcessed = true
	}

	// GH-Cp gen: Check if the input file exists locally and transfer it to server if needed
	if inFilePath != "" && utils.FileExists(inFilePath) {
		logger.Verbose("Input file found locally: %s", inFilePath)
		
		// Mark this as the primary input file
		isPrimaryInputFile[inFilePath] = true

		// Transfer file to server and get the server-side path
		serverPath, err := sendFileToServer(inFilePath)
		if err != nil {
			logger.Error("Error transferring file %s: %v", inFilePath, err)
			// Set failure flag
			*aviFail = C.int(1)
			// Set error message
			errMsg := fmt.Sprintf("File transfer failed: %v", err)
			copy((*[1 << 24]byte)(unsafe.Pointer(avcMsg))[:msgSize], []byte(errMsg))
			return
		}

		// GH-Cp gen: Update the InFile field in payload with the server-side path
		// First, create a new byte slice with the modified path
		serverPathBytes := []byte(serverPath + "\x00")

		// Copy to payload.InFile
		payload.InFile = serverPathBytes

		// Update the inFileSize in the swap array
		swap[49] = float32(len(serverPathBytes))

		logger.Verbose("Using server path for input file: %s", serverPath)
	} else {
		// Handle original path - with safety checks
		if accInFile != nil && inFileSize > 0 {
			if inFilePath != "" {
				logger.Debug("Input file not found locally: %s, continuing with original path", inFilePath)
			}
			payload.InFile = (*[1 << 24]byte)(unsafe.Pointer(accInFile))[:inFileSize:inFileSize]
		} else {
			// Ensure we have at least an empty byte array with a null terminator
			payload.InFile = []byte{0}
		}
	}

	// Fill the rest of the payload
	payload.Swap = swap[:swapSize:swapSize]
	payload.Fail = int32(*aviFail)

	// Safely handle output name and message with null checks
	if avcOutName != nil && outNameSize > 0 {
		payload.OutName = (*[1 << 24]byte)(unsafe.Pointer(avcOutName))[:outNameSize:outNameSize]
	} else {
		payload.OutName = []byte{0}
	}

	if avcMsg != nil && msgSize > 0 {
		payload.Msg = (*[1 << 24]byte)(unsafe.Pointer(avcMsg))[:msgSize:msgSize]
	} else {
		payload.Msg = []byte{0}
	}

	// Reset file transfer fields to avoid sending unnecessary data
	payload.FileContent = nil
	payload.ServerFilePath = nil

	// GH-Cp gen: Ensure payload is properly formatted
	utils.PreparePayloadForTransmission(&payload)

	// Convert payload to binary and send over websocket
	b, err := payload.MarshalBinary()
	if err != nil {
		log.Fatalf("discon-client: %s", err)
	}
	ws.WriteMessage(websocket.BinaryMessage, b)

	logger.Verbose("sent payload: %v", payload)

	if debugLevel >= 1 && sentSwapFile != nil {
		outSwapSize := min(swapSize, 163)
		for _, v := range payload.Swap[:outSwapSize-1] {
			fmt.Fprintf(sentSwapFile, "%g,", v)
		}
		fmt.Fprintf(sentSwapFile, "%g\n", payload.Swap[outSwapSize-1])
	}

	// Read response from server
	_, b, err = ws.ReadMessage()
	if err != nil {
		log.Fatalf("discon-client: %s", err)
	}

	// Unmarshal binary data into payload
	err = payload.UnmarshalBinary(b)
	if err != nil {
		log.Fatalf("discon-client: %s", err)
	}

	logger.Verbose("received payload: %v", payload)

	if debugLevel >= 1 && recvSwapFile != nil {
		outSwapSize := min(swapSize, 163)
		for _, v := range payload.Swap[:outSwapSize-1] {
			fmt.Fprintf(recvSwapFile, "%g,", v)
		}
		fmt.Fprintf(recvSwapFile, "%g\n", payload.Swap[outSwapSize-1])
	}

	// Set fail flag
	*aviFail = C.int(payload.Fail)
}

func main() {}
