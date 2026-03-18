// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tts

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// R2Storage implements AudioStorage for Cloudflare R2.
type R2Storage struct {
	client      *s3.Client
	presigner   *s3.PresignClient
	bucket      string
	publicURL   string
}

// newR2Storage creates a new R2 storage backend.
func newR2Storage(config *StorageConfig) (*R2Storage, error) {
	// Validate required fields
	if config.R2Endpoint == "" {
		return nil, fmt.Errorf("R2 endpoint is required")
	}
	if config.R2AccessKeyID == "" {
		return nil, fmt.Errorf("R2 access key ID is required")
	}
	if config.R2SecretAccessKey == "" {
		return nil, fmt.Errorf("R2 secret access key is required")
	}
	if config.R2Bucket == "" {
		return nil, fmt.Errorf("R2 bucket is required")
	}

	// Create AWS config for R2
	awsConfig := aws.Config{
		Region: "auto", // R2 uses "auto" as region
		Credentials: credentials.NewStaticCredentialsProvider(
			config.R2AccessKeyID,
			config.R2SecretAccessKey,
			"",
		),
		EndpointResolverWithOptions: aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL:               config.R2Endpoint,
					SigningRegion:     "auto",
					HostnameImmutable: true,
				}, nil
			},
		),
	}

	// Create S3 client
	client := s3.NewFromConfig(awsConfig)

	// Create presigner
	presigner := s3.NewPresignClient(client)

	// Use public URL if provided, otherwise use endpoint
	publicURL := config.R2PublicURL
	if publicURL == "" {
		publicURL = config.R2Endpoint
	}

	return &R2Storage{
		client:    client,
		presigner: presigner,
		bucket:    config.R2Bucket,
		publicURL: publicURL,
	}, nil
}

// Save uploads audio data to R2.
func (s *R2Storage) Save(data []byte, path string) error {
	ctx := context.Background()

	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(path),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("audio/mpeg"),
	})

	if err != nil {
		return fmt.Errorf("failed to upload to R2: %w", err)
	}

	return nil
}

// GetURL generates a presigned URL for accessing the audio file.
func (s *R2Storage) GetURL(path string, expiresAt time.Time) (string, error) {
	ctx := context.Background()

	// Calculate expiration duration
	expiresIn := time.Until(expiresAt)
	if expiresIn < 0 {
		return "", fmt.Errorf("expiration time is in the past")
	}

	// Generate presigned URL
	req, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = expiresIn
	})

	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return req.URL, nil
}

// Delete removes the audio file from R2.
func (s *R2Storage) Delete(path string) error {
	ctx := context.Background()

	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(path),
	})

	if err != nil {
		return fmt.Errorf("failed to delete from R2: %w", err)
	}

	return nil
}
