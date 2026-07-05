package external

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/lsj/copylingo/internal/config"
)

// AudioContentType is the MIME type stored for OGG/Opus voice clips.
const AudioContentType = "audio/ogg"

// AudioStore is the object-store contract for TTS audio (ADR-032). The single
// implementation targets any S3-compatible backend (local MinIO, prod AWS S3)
// via an endpoint swap.
type AudioStore interface {
	// Exists reports whether an object already exists at key (dedup check).
	Exists(ctx context.Context, key string) (bool, error)
	// Put stores body at key with the given content type.
	Put(ctx context.Context, key string, body []byte, contentType string) error
	// Get fetches the object bytes at key.
	Get(ctx context.Context, key string) ([]byte, error)
}

// AudioKey builds the content-addressed object key for a synthesized clip:
// tts/{lang}/{voice}/{sha256(script)}.ogg (ADR-032). Identical scripts collapse
// to one object regardless of which question referenced them.
func AudioKey(language, voice, script string) string {
	sum := sha256.Sum256([]byte(script))
	return fmt.Sprintf("tts/%s/%s/%x.ogg", keySegment(language), keySegment(voice), sum)
}

func keySegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "unknown"
	}
	return s
}

// S3AudioStore is the aws-sdk-go-v2 implementation of AudioStore.
type S3AudioStore struct {
	client *s3.Client
	bucket string
}

// NewS3AudioStore constructs an S3 client from static credentials and an optional
// custom endpoint (empty => AWS default for the region). Path-style addressing is
// required by MinIO and harmless to leave off for AWS.
func NewS3AudioStore(cfg *config.Config) *S3AudioStore {
	awsCfg := aws.Config{
		Region:      cfg.Storage.Region,
		Credentials: credentials.NewStaticCredentialsProvider(cfg.Storage.AccessKey, cfg.Storage.SecretKey, ""),
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint := strings.TrimSpace(cfg.Storage.Endpoint); endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = cfg.Storage.UsePathStyle
	})

	return &S3AudioStore{client: client, bucket: cfg.Storage.Bucket}
}

func (s *S3AudioStore) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("audio store head %q: %w", key, err)
	}
	return true, nil
}

func (s *S3AudioStore) Put(ctx context.Context, key string, body []byte, contentType string) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(body),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return fmt.Errorf("audio store put %q: %w", key, err)
	}
	return nil
}

func (s *S3AudioStore) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("audio store get %q: %w", key, err)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("audio store read %q: %w", key, err)
	}
	return data, nil
}

// isNotFound recognizes the S3 "object absent" errors across backends so Exists
// can return (false, nil) instead of surfacing them.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}
	return false
}
