package avatar

import (
	"bytes"
	"context"
	stderrors "errors"
	"io"
	"slices"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

// S3 keeps avatars in a bucket, which it creates when it is missing.
type S3 struct {
	client *s3.Client
	bucket string

	mu      sync.Mutex
	created bool
}

// NewS3 keeps avatars in the bucket.
func NewS3(client *s3.Client, bucket string) *S3 {
	return &S3{client: client, bucket: bucket}
}

// Put stores the image, creating the bucket first if it doesn't exist yet.
func (s *S3) Put(ctx context.Context, key string, data []byte, contentType string) error {
	put := func() error {
		_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			Body:        bytes.NewReader(data),
			ContentType: aws.String(contentType),
		})

		return err
	}

	err := put()
	if !hasCode(err, "NoSuchBucket") {
		return err
	}

	if err := s.createBucket(ctx); err != nil {
		return err
	}

	return put()
}

// Get returns the image and its type.
func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if hasCode(err, "NoSuchKey", "NoSuchBucket", "NotFound") {
		return nil, "", ErrNotFound
	}

	if err != nil {
		return nil, "", err
	}

	return out.Body, aws.ToString(out.ContentType), nil
}

// Delete removes the image; one that is gone already is fine.
func (s *S3) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if hasCode(err, "NoSuchKey", "NoSuchBucket", "NotFound") {
		return nil
	}

	return err
}

// createBucket creates the bucket once, accepting one that another server created.
func (s *S3) createBucket(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.created {
		return nil
	}

	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	if err != nil && !hasCode(err, "BucketAlreadyOwnedByYou", "BucketAlreadyExists") {
		return err
	}

	s.created = true

	return nil
}

// hasCode tells whether err is an S3 error with one of the codes.
func hasCode(err error, codes ...string) bool {
	var apiErr smithy.APIError

	return stderrors.As(err, &apiErr) && slices.Contains(codes, apiErr.ErrorCode())
}
