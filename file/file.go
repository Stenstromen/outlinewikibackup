package file

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func UploadToS3(filename string) error {
	var cfg aws.Config
	var err error

	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
			endpoint = "https://" + endpoint
		}

		cfg, err = config.LoadDefaultConfig(context.TODO(),
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
		)
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO(),
			config.WithRegion(os.Getenv("AWS_REGION")),
		)
	}

	if err != nil {
		return fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	s3Client := s3.NewFromConfig(cfg)

	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("unable to open file %q: %w", filename, err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	var size = fileInfo.Size()
	buffer := make([]byte, size)
	file.Read(buffer)

	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(os.Getenv("S3_BUCKET_NAME")),
		Key:    aws.String(filepath.Base(filename)),
		Body:   bytes.NewReader(buffer),
		ACL:    types.ObjectCannedACLPrivate,
	})
	if err != nil {
		return fmt.Errorf("unable to upload %q to %q: %w", filename, os.Getenv("S3_BUCKET_NAME"), err)
	}

	log.Println("Successfully uploaded", filename, "to", os.Getenv("S3_BUCKET_NAME"))
	return nil
}

func KeepOnlyNBackups(keepBackups string) error {
	s3env := os.Getenv("UPLOAD_TO_S3")
	keepBackupsInt, err := strconv.Atoi(keepBackups)
	if err != nil {
		panic(err)
	}
	if s3env == "true" {
		var cfg aws.Config
		var err error

		if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
			if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
				endpoint = "https://" + endpoint
			}

			cfg, err = config.LoadDefaultConfig(context.TODO(),
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
			)
		} else {
			cfg, err = config.LoadDefaultConfig(context.TODO(),
				config.WithRegion(os.Getenv("AWS_REGION")),
			)
		}

		if err != nil {
			return fmt.Errorf("unable to load AWS SDK config: %w", err)
		}

		s3Client := s3.NewFromConfig(cfg)

		resp, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
			Bucket: aws.String(os.Getenv("S3_BUCKET_NAME")),
		})
		if err != nil {
			return fmt.Errorf("unable to list objects in bucket %q: %w", os.Getenv("S3_BUCKET_NAME"), err)
		}

		sortS3ObjectsByLastModified(resp.Contents)

		numObjects := len(resp.Contents)
		numToDelete := numObjects - keepBackupsInt
		if numToDelete > 0 {
			objectsToDelete := resp.Contents[:numToDelete]
			for _, obj := range objectsToDelete {
				_, err := s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
					Bucket: aws.String(os.Getenv("S3_BUCKET_NAME")),
					Key:    obj.Key,
				})
				if err != nil {
					return fmt.Errorf("unable to delete object %q: %w", *obj.Key, err)
				}
				log.Println("Deleted object:", *obj.Key)
			}
		}
	} else {
		saveDir := os.Getenv("SAVE_DIR")
		if saveDir == "" {
			saveDir = "/tmp/outlinewikibackups"
		}

		files, err := os.ReadDir(saveDir)
		if err != nil {
			return fmt.Errorf("unable to list files in directory %q: %w", saveDir, err)
		}

		sortFilesByLastModified(files)

		numFiles := len(files)
		numToDelete := numFiles - keepBackupsInt
		if numToDelete > 0 {
			filesToDelete := files[:numToDelete]
			for _, file := range filesToDelete {
				if err := os.Remove(filepath.Join(saveDir, file.Name())); err != nil {
					return fmt.Errorf("unable to delete file %q: %w", file.Name(), err)
				}
				log.Println("Deleted file:", file.Name())
			}
		}
	}
	return nil
}

func sortS3ObjectsByLastModified(objects []types.Object) {
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].LastModified.Before(*objects[j].LastModified)
	})
}

func sortFilesByLastModified(files []os.DirEntry) {
	sort.Slice(files, func(i, j int) bool {
		file1, _ := files[i].Info()
		file2, _ := files[j].Info()
		return file1.ModTime().Before(file2.ModTime())
	})
}
