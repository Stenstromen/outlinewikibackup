package s3api

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
)

func GetConfig() aws.Config {
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
		if err != nil {
			log.Fatal("Failed to create MinIO config:", err)
		}
		return cfg
	}
	// TODO: Garage really cares only about signing region, so
	// almost same as minio - merge?
	if endpoint := os.Getenv("GARAGE_ENDPOINT"); endpoint != "" {
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
						SigningRegion:     "garage",
						HostnameImmutable: true,
					}, nil
				},
			)),
			config.WithRequestChecksumCalculation(aws.RequestChecksumCalculationWhenRequired),
		)
		if err != nil {
			log.Fatal("Failed to create Garage config:", err)
		}
		return cfg
	}
	cfg, err = config.LoadDefaultConfig(context.Background(),
		config.WithRegion(os.Getenv("AWS_REGION")),
	)

	if err != nil {
		log.Fatal("Failed to create S3 config:", err)
	}
	return cfg
}
