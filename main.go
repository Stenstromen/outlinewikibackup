package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stenstromen/outlinewikibackup/api"
	"github.com/stenstromen/outlinewikibackup/file"
)

func init() {
	// Enable container-aware GOMAXPROCS for better performance in containers
	// This will automatically adjust based on cgroup CPU limits
	runtime.SetDefaultGOMAXPROCS()

	if _, exists := os.LookupEnv("API_BASE_URL"); !exists {
		log.Fatal("API_BASE_URL environment variable is not set.")
	}

	if _, exists := os.LookupEnv("AUTH_TOKEN"); !exists {
		log.Fatal("AUTH_TOKEN environment variable is not set.")
	}

	saveDir := os.Getenv("SAVE_DIR")
	if saveDir == "" {
		saveDir = "/tmp/outlinewikibackups"
	}

	if err := os.MkdirAll(saveDir, os.ModePerm); err != nil {
		log.Fatal("Unable to create save directory:", err)
	}

	testFile := filepath.Join(saveDir, "test_write")
	if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
		log.Fatal("Save directory is not writable:", err)
	}
	os.Remove(testFile)

	// Check if API endpoint is reachable
	apiBaseURL := os.Getenv("API_BASE_URL")
	client := &http.Client{Timeout: 5 * time.Second}
	_, err := client.Get(apiBaseURL)
	if err != nil {
		log.Fatal("API endpoint is not reachable:", err)
	}

	// Check S3/MinIO connectivity if UPLOAD_TO_S3 is enabled
	if os.Getenv("UPLOAD_TO_S3") == "true" {
		var cfg aws.Config
		var err error

		if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				endpoint = "https://" + endpoint
			}

			cfg, err = config.LoadDefaultConfig(context.Background(),
				config.WithRegion("us-east-1"),
				config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
					os.Getenv("AWS_ACCESS_KEY_ID"),
					os.Getenv("AWS_SECRET_ACCESS_KEY"),
					"",
				)),
				config.WithEndpointResolver(aws.EndpointResolverFunc(
					func(service, region string) (aws.Endpoint, error) {
						return aws.Endpoint{
							PartitionID:       "aws",
							URL:               endpoint,
							SigningRegion:     "us-east-1",
							HostnameImmutable: true,
						}, nil
					},
				)),
				config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
			)
		} else {
			cfg, err = config.LoadDefaultConfig(context.Background(),
				config.WithRegion(os.Getenv("AWS_REGION")),
			)
		}

		if err != nil {
			log.Fatal("Failed to create S3/MinIO config:", err)
		}

		// Skip ListBuckets check if MINIMAL_S3_PERMISSIONS is set to "true"
		if os.Getenv("MINIMAL_S3_PERMISSIONS") != "true" {
			// Try to list buckets to verify connectivity
			s3Client := s3.NewFromConfig(cfg)
			_, err = s3Client.ListBuckets(context.Background(), &s3.ListBucketsInput{})
			if err != nil {
				log.Fatal("S3/MinIO is not reachable:", err)
			}
		} else {
			log.Println("S3/MinIO connectivity check disabled via MINIMAL_S3_PERMISSIONS")
		}
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
