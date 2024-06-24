package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stenstromen/outlinewikibackup/types"
)

const (
	exportEndpoint   = "/api/collections.export_all"
	progressEndpoint = "/api/fileOperations.info"
	downloadEndpoint = "/api/fileOperations.redirect"
	deleteEndpoint   = "/api/fileOperations.delete"
)

var apiBaseURL string

func init() {
	var ok bool
	apiBaseURL, ok = os.LookupEnv("API_BASE_URL")
	if !ok {
		log.Fatal("API_BASE_URL environment variable is not set.")
	}

	_, err := url.ParseRequestURI(apiBaseURL)
	if err != nil {
		log.Fatalf("API_BASE_URL is not a valid URL: %v", err)
	}
}

func makeAPIRequest(endpoint string, payload map[string]string) (resp *http.Response, err error) {
	authToken := os.Getenv("AUTH_TOKEN")

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("Error marshalling payload:", err)
		return nil, err
	}

	req, err := http.NewRequest("POST", apiBaseURL+endpoint, bytes.NewBuffer(body))
	if err != nil {
		log.Println("Error creating request:", err)
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+authToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		log.Println("Error sending request:", err)
		return nil, err
	}

	return resp, nil
}

func InitiateExport() (string, error) {
	payload := map[string]string{
		"format": "outline-markdown",
	}

	resp, err := makeAPIRequest(exportEndpoint, payload)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var exportResp types.ExportResponse
	if err := json.NewDecoder(resp.Body).Decode(&exportResp); err != nil {
		log.Println("Error decoding response:", err)
		return "", err
	}

	if !exportResp.Success {
		log.Println("Failed to initiate export")
		return "", fmt.Errorf("failed to initiate export")
	}

	return exportResp.Data.FileOperation.ID, nil
}

func WaitForExportCompletion(exportID string) error {
	for {
		defaultSleepDuration := 10
		sleepEnv := os.Getenv("SLEEP_DURATION")
		sleepDuration, err := strconv.Atoi(sleepEnv)
		if err != nil {
			sleepDuration = defaultSleepDuration
		}
		time.Sleep(time.Duration(sleepDuration) * time.Second)

		reqBody := map[string]string{
			"id": exportID,
		}

		resp, err := makeAPIRequest(progressEndpoint, reqBody)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		var progressResp types.ProgressResponse
		if err := json.NewDecoder(resp.Body).Decode(&progressResp); err != nil {
			log.Println("Error decoding response:", err)
			return err
		}

		log.Println("Export state:", progressResp.Data.State)

		if progressResp.Data.State == "complete" {
			return nil
		}

		log.Println("Export is still in progress, waiting...")
	}
}

func FetchAndSaveExport(exportID string) (string, error) {
	reqBody := map[string]string{
		"id": exportID,
	}

	resp, err := makeAPIRequest(downloadEndpoint, reqBody)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Failed to get export file")
		return "", fmt.Errorf("failed to get export file")
	}

	parsedURL, err := url.Parse(apiBaseURL)
	if err != nil {
		log.Println("Error parsing API_BASE_URL:", err)
		return "", err
	}
	hostname := parsedURL.Hostname()
	currentTime := time.Now().Format(time.RFC3339)
	filename := fmt.Sprintf("%s-outline-backup-%s.zip", hostname, currentTime)

	saveDir := os.Getenv("SAVE_DIR")
	if saveDir == "" {
		saveDir = "/tmp/outlinewikibackups"
	}

	fullPath := filepath.Join(saveDir, filename)

	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Println("Error creating save directory:", err)
		return "", err
	}

	out, err := os.Create(fullPath)
	if err != nil {
		log.Println("Error creating file:", err)
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		log.Println("Error saving file:", err)
		return "", err
	}

	log.Println("File saved as:", fullPath)

	return fullPath, nil
}

func DeleteExport(exportID string) error {
	reqBody := map[string]string{
		"id": exportID,
	}

	resp, err := makeAPIRequest(deleteEndpoint, reqBody)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Println("Failed to delete export")
		return fmt.Errorf("failed to delete export")
	}

	log.Println("Export deletion response received")

	return nil
}
