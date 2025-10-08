package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

type ExportResponse struct {
	Success bool `json:"success"`
	Data    struct {
		FileOperation struct {
			ID     string `json:"id"`
			State  string `json:"state"`
			Name   string `json:"name"`
			Format string `json:"format"`
		} `json:"fileOperation"`
	} `json:"data"`
	Status int  `json:"status"`
	Ok     bool `json:"ok"`
}

type ProgressResponse struct {
	Data struct {
		ID     string `json:"id"`
		State  string `json:"state"`
		Format string `json:"format"`
		Name   string `json:"name"`
	} `json:"data"`
	Status int  `json:"status"`
	Ok     bool `json:"ok"`
}

var exportStates = make(map[string]string)
var exportCounter = 0

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.HandleFunc("/api/collections.export_all", handleExportAll)
	http.HandleFunc("/api/fileOperations.info", handleFileOperationInfo)
	http.HandleFunc("/api/fileOperations.redirect", handleFileOperationRedirect)
	http.HandleFunc("/api/fileOperations.delete", handleFileOperationDelete)
	http.HandleFunc("/health", handleHealth)

	log.Printf("Mock Outline server starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func handleExportAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || authHeader != "Bearer test-token" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	exportCounter++
	exportID := fmt.Sprintf("export-%d", exportCounter)

	// Initialize export state as "processing"
	exportStates[exportID] = "processing"

	response := ExportResponse{
		Success: true,
		Data: struct {
			FileOperation struct {
				ID     string `json:"id"`
				State  string `json:"state"`
				Name   string `json:"name"`
				Format string `json:"format"`
			} `json:"fileOperation"`
		}{
			FileOperation: struct {
				ID     string `json:"id"`
				State  string `json:"state"`
				Name   string `json:"name"`
				Format string `json:"format"`
			}{
				ID:     exportID,
				State:  "processing",
				Name:   "outline-backup.zip",
				Format: "outline-markdown",
			},
		},
		Status: 200,
		Ok:     true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	// Simulate async processing - mark as complete after a delay
	go func() {
		time.Sleep(2 * time.Second)
		exportStates[exportID] = "complete"
	}()
}

func handleFileOperationInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || authHeader != "Bearer test-token" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var requestBody struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	exportID := requestBody.ID
	state, exists := exportStates[exportID]
	if !exists {
		http.Error(w, "Export not found", http.StatusNotFound)
		return
	}

	response := ProgressResponse{
		Data: struct {
			ID     string `json:"id"`
			State  string `json:"state"`
			Format string `json:"format"`
			Name   string `json:"name"`
		}{
			ID:     exportID,
			State:  state,
			Format: "outline-markdown",
			Name:   "outline-backup.zip",
		},
		Status: 200,
		Ok:     true,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handleFileOperationRedirect(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || authHeader != "Bearer test-token" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var requestBody struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	exportID := requestBody.ID
	state, exists := exportStates[exportID]
	if !exists {
		http.Error(w, "Export not found", http.StatusNotFound)
		return
	}

	if state != "complete" {
		http.Error(w, "Export not ready", http.StatusBadRequest)
		return
	}

	// Create a fake ZIP file content
	fakeZipContent := []byte("PK\x03\x04\x14\x00\x00\x00\x08\x00\x00\x00\x00\x00fake-outline-backup-content")

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=outline-backup.zip")
	w.Header().Set("Content-Length", strconv.Itoa(len(fakeZipContent)))

	w.Write(fakeZipContent)
}

func handleFileOperationDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check authorization
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" || authHeader != "Bearer test-token" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var requestBody struct {
		ID string `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	exportID := requestBody.ID
	if _, exists := exportStates[exportID]; !exists {
		http.Error(w, "Export not found", http.StatusNotFound)
		return
	}

	// Delete the export
	delete(exportStates, exportID)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Export deleted successfully"))
}
