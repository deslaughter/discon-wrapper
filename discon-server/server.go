package main

import (
	"discon-wrapper/shared/utils"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

const program = "discon-server"
const version = "v0.2.0"

// GH-Cp gen: Using an int for debug levels instead of boolean
var debugLevel int = 0
var port = 8080
var serverLogger *utils.DebugLogger

func main() {
	// Configure logging with timestamp, file and line number
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	// Display program and version
	log.Printf("Started %s %s", program, version)

	flag.IntVar(&port, "port", 8080, "Port to listen on")
	// GH-Cp gen: Updated to use debug levels
	flag.IntVar(&debugLevel, "debug", 0, "Debug level: 0=disabled, 1=basic info, 2=verbose with payloads")
	flag.Parse()
	
	// Create server-wide logger for non-connection-specific logs
	serverLogger = utils.NewDebugLogger(debugLevel, "discon-server")
	
	serverLogger.Debug("Server initialized with debug level %d", debugLevel)
	serverLogger.Debug("Hostname: %s", getHostname())

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serverLogger.Debug("New connection request from %s", r.RemoteAddr)
		start := time.Now()
		ServeWs(w, r, debugLevel)
		connectionDuration := time.Since(start)
		serverLogger.Debug("Connection from %s closed after %v", r.RemoteAddr, connectionDuration)
	})

	// Start server
	serverLogger.Debug("Listening on port %d", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		serverLogger.Error("ListenAndServe failed: %v", err)
		log.Fatal("ListenAndServe: ", err)
	}
}

// Helper function to get hostname for better log context
func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown-host"
	}
	return hostname
}
