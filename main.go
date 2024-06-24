package main

import (
	"log"
	"os"

	"github.com/stenstromen/outlinewikibackup/api"
	"github.com/stenstromen/outlinewikibackup/file"
)

func init() {
	if _, exists := os.LookupEnv("API_BASE_URL"); !exists {
		log.Fatal("API_BASE_URL environment variable is not set.")
	}

	if _, exists := os.LookupEnv("AUTH_TOKEN"); !exists {
		log.Fatal("AUTH_TOKEN environment variable is not set.")
	}
}

func main() {
	log.Println("Starting Outline Wiki Backup...")

	exportID, err := api.InitiateExport()
	if err != nil {
		log.Println("Error initiating export:", err)
		return
	}
	log.Println("Export initiated, ID:", exportID)

	log.Println("Checking export progress...")
	err = api.WaitForExportCompletion(exportID)
	if err != nil {
		log.Println("Error checking export progress:", err)
		return
	}
	log.Println("Export completed!")

	log.Println("Fetching download link and saving file...")
	filename, err := api.FetchAndSaveExport(exportID)
	if err != nil {
		log.Println("Error fetching and saving export:", err)
		return
	}
	log.Println("File downloaded successfully:", filename)

	uploadToS3Flag := os.Getenv("UPLOAD_TO_S3")
	if uploadToS3Flag == "true" {
		log.Println("Uploading file to S3/MinIO...")
		err = file.UploadToS3(filename)
		if err != nil {
			log.Println("Error uploading file to S3/MinIO:", err)
			return
		}
		log.Println("File uploaded successfully to S3/MinIO")
		if err := os.Remove(filename); err != nil {
			log.Println("Error deleting file:", err)
			return
		}
		log.Println("Local file deleted successfully")
	}

	log.Println("Deleting export from server...")
	err = api.DeleteExport(exportID)
	if err != nil {
		log.Println("Error deleting export:", err)
		return
	}
	log.Println("Export deleted successfully!")

	keepBackups := os.Getenv("KEEP_BACKUPS")
	if keepBackups != "" {
		log.Println("Keeping only", keepBackups, "backups")
		err = file.KeepOnlyNBackups(keepBackups)
		if err != nil {
			log.Println("Error keeping only", keepBackups, "backups:", err)
			return
		}
	} else {
		log.Println("Keeping all backups")
	}

	log.Println("Backup completed successfully!")
}
