package main

import (
	dw "discon-wrapper"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"testing"

	"github.com/gorilla/websocket"
)

func TestServeWs(t *testing.T) {

	const port = 18080

	// connect handler to websocket function
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ServeWs(w, r, 1)
	})

	// Start server in separate go routine
	go func() {
		log.Printf("Listening on port %d", port)
		err := http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}()

	// Create a URL object
	u, err := url.Parse(fmt.Sprintf("ws://localhost:%d/ws", port))
	if err != nil {
		log.Fatal(err)
		return
	}

	// Add query parameters for shared library path and proc
	q := u.Query()
	q.Add("path", "../build/test-discon.dll")
	q.Add("proc", "discon")
	u.RawQuery = q.Encode()

	// Connect to websocket server
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// Create payload
	payload := dw.Payload{
		Swap:    make([]float32, 130),
		Fail:    1,
		InFile:  []byte("input.txt\u0000"),
		OutName: []byte("output.txt\u0000"),
		Msg:     []byte("Hello, World!        \u0000"),
	}
	payload.Swap[48] = float32(len(payload.Msg))
	payload.Swap[49] = float32(len(payload.InFile))
	payload.Swap[50] = float32(len(payload.OutName))
	payload.Swap[128] = float32(len(payload.Swap))

	// Convert payload to binary and send over websocket
	b, err := payload.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	ws.WriteMessage(websocket.BinaryMessage, b)

	t.Log("Client sent payload:", payload)

	// Clear data in payload
	payload.Swap = make([]float32, len(payload.Swap))
	payload.Fail = -1
	payload.InFile = make([]byte, len(payload.InFile))
	payload.OutName = make([]byte, len(payload.OutName))
	payload.Msg = make([]byte, len(payload.Msg))

	// Read response from server
	_, b, err = ws.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}

	// Unmarshal binary data into payload
	err = payload.UnmarshalBinary(b)
	if err != nil {
		t.Fatal(err)
	}

	// Print payload
	t.Log("Client received payload:", payload)

	// Check if the payload is correct
	if payload.Fail != 0 {
		t.Errorf("Expected Fail to be 0, got %d", payload.Fail)
	}
	if string(payload.InFile) != "input.txt\u0000" {
		t.Errorf("Expected InFile to be 'input.txt\\u0000', got '%s'", string(payload.InFile))
	}
	if string(payload.OutName) != "output.txt\u0000" {
		t.Errorf("Expected OutName to be 'output.txt\\u0000', got '%s'", string(payload.OutName))
	}
	if string(payload.Msg) != "DISCON called 1 times\u0000" {
		t.Errorf("Expected Msg to be 'DISCON called 1 times\\u0000', got '%s'", string(payload.Msg))
	}
	if len(payload.Swap) != 130 {
		t.Errorf("Expected Swap to be of length 130, got %d", len(payload.Swap))
	}
	if payload.Swap[48] != float32(len(payload.Msg)) {
		t.Errorf("Expected Swap[48] to be %f, got %f", float32(len(payload.Msg)), payload.Swap[48])
	}
	if payload.Swap[49] != float32(len(payload.InFile)) {
		t.Errorf("Expected Swap[49] to be %f, got %f", float32(len(payload.InFile)), payload.Swap[49])
	}
	if payload.Swap[50] != float32(len(payload.OutName)) {
		t.Errorf("Expected Swap[50] to be %f, got %f", float32(len(payload.OutName)), payload.Swap[50])
	}
	if payload.Swap[128] != float32(len(payload.Swap)) {
		t.Errorf("Expected Swap[128] to be %f, got %f", float32(len(payload.Swap)), payload.Swap[128])
	}

	// Close connection
	ws.Close()

	// Wait for server to finish
	wg.Wait()
}
