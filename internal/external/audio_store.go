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
)

// AudioContentType is the MIME type stored for OGG/Opus voice clips.
const AudioContentType = "audio/ogg"

// AudioKey builds the content-addressed object key for a synthesized clip:
// tts/{lang}/{voice}/{sha256(script)}.ogg (ADR-032). Identical scripts collapse
// to one object regardless of which question referenced them.
func AudioKey(
	language,
	voice,
	script string,
) string {
	sum := sha256.Sum256([]byte(script))
	return fmt.Sprintf(
		"tts/%s/%s/%x.ogg",
		keySegment(language),
		keySegment(voice),
		sum,
	)
}

func keySegment(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "unknown"
	}
	return s
}

// S3AudioStore is the object store for TTS audio (ADR-032), targeting any
// S3-compatible backend (local MinIO, prod AWS S3) via an endpoint swap.
// Built on aws-sdk-go-v2.
type S3AudioStore struct {
	client *s3.Client
	bucket string
}

// S3Options are the settings NewS3AudioStore needs. An empty Endpoint uses the
// AWS default for Region; UsePathStyle is true for MinIO.
type S3Options struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// NewS3AudioStore constructs an S3 client from static credentials and an optional
// custom endpoint (empty => AWS default for the region). Path-style addressing is
// required by MinIO and harmless to leave off for AWS.
func NewS3AudioStore(opts S3Options) *S3AudioStore {
	awsCfg := aws.Config{
		Region: opts.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			opts.AccessKey,
			opts.SecretKey,
			"",
		),
	}

	client := s3.NewFromConfig(
		awsCfg,
		func(o *s3.Options) {
			if endpoint := strings.TrimSpace(opts.Endpoint); endpoint != "" {
				o.BaseEndpoint = aws.String(endpoint)
			}
			o.UsePathStyle = opts.UsePathStyle
		},
	)

	return &S3AudioStore{client: client, bucket: opts.Bucket}
}

func (s *S3AudioStore) Exists(
	ctx context.Context,
	key string,
) (bool, error) {
	_, err := s.client.HeadObject(
		ctx,
		&s3.HeadObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
	)
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf(
			"audio store head %q: %w",
			key,
			err,
		)
	}
	return true, nil
}

func (s *S3AudioStore) Put(
	ctx context.Context,
	key string,
	body []byte,
	contentType string,
) error {
	_, err := s.client.PutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(body),
			ContentType: aws.String(contentType),
		},
	)
	if err != nil {
		return fmt.Errorf(
			"audio store put %q: %w",
			key,
			err,
		)
	}
	return nil
}

func (s *S3AudioStore) Get(
	ctx context.Context,
	key string,
) ([]byte, error) {
	out, err := s.client.GetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
	)
	if err != nil {
		return nil, fmt.Errorf(
			"audio store get %q: %w",
			key,
			err,
		)
	}
	defer out.Body.Close()

	data, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"audio store read %q: %w",
			key,
			err,
		)
	}
	return data, nil
}

// isNotFound recognizes the S3 "object absent" errors across backends so Exists
// can return (false, nil) instead of surfacing them.
func isNotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(
		err,
		&apiErr,
	) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}
	return false
}
