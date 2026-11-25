package objectstore

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

const (
	// defaultRegion is the region of storages that have none, as most S3-compatible
	// ones accept it.
	defaultRegion = "us-east-1"
	// shaMetadata is the metadata key of an object's SHA-256, sent as x-amz-meta-sha256.
	shaMetadata = "sha256"
	// checkKey is the object Check writes under its prefix.
	checkKey = ".parlance-check"
	// maxGetSize caps what Get reads; only manifests are read back.
	maxGetSize = 16 << 20
)

// S3 is a bucket on Amazon S3 or an S3-compatible storage, such as MinIO, Cloudflare
// R2 or Backblaze B2. Its key needs to list the bucket, so that missing objects show
// as missing, and to read and write objects; deleting is optional.
type S3 struct {
	client *s3.Client
	bucket string
}

// NewS3 connects to the bucket; client, if not nil, sends the requests.
func NewS3(config Config, client *http.Client) *S3 {
	region := config.Region
	if region == "" {
		region = defaultRegion
	}

	options := s3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(config.AccessKeyID, config.SecretAccessKey, ""),
		UsePathStyle: config.PathStyle,
		// Several S3-compatible storages reject the checksums the SDK adds by default.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
	}

	if config.Endpoint != "" {
		options.BaseEndpoint = aws.String(config.Endpoint)
	}

	if client != nil {
		options.HTTPClient = client
	}

	return &S3{client: s3.New(options), bucket: config.Bucket}
}

// Check tells whether the bucket exists and can be written under the prefix. The
// object it writes stays when the key may not delete; the next check replaces it.
func (s *S3) Check(ctx context.Context, prefix string) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil {
		if statusCode(err) == http.StatusNotFound {
			return fmt.Errorf("the bucket %q does not exist", s.bucket)
		}

		return s.explain(err)
	}

	key := path.Join(prefix, checkKey)

	if err := s.Put(ctx, Object{Key: key, Body: []byte("ok\n"), ContentType: "text/plain"}); err != nil {
		return fmt.Errorf("can't write %s: %w", key, err)
	}

	//nolint:errcheck // Keys may write without deleting; the next check replaces the object.
	_, _ = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})

	return nil
}

// Stat tells whether the bucket holds an object at the key, and its SHA-256.
func (s *S3) Stat(ctx context.Context, key string) (Stat, error) {
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if statusCode(err) == http.StatusNotFound {
		return Stat{}, nil
	}

	if err != nil {
		return Stat{}, s.explain(err)
	}

	return Stat{Exists: true, SHA256: out.Metadata[shaMetadata]}, nil
}

// Get returns the object at the key, or ErrNotFound.
func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if hasCode(err, "NoSuchKey") {
		return nil, ErrNotFound
	}

	if err != nil {
		return nil, s.explain(err)
	}

	defer out.Body.Close()

	data, err := io.ReadAll(io.LimitReader(out.Body, maxGetSize+1))
	if err != nil {
		return nil, s.explain(err)
	}

	if len(data) > maxGetSize {
		return nil, fmt.Errorf("%s is larger than %d MB", key, maxGetSize>>20)
	}

	return data, nil
}

// Put stores the object, with its SHA-256 in the metadata if it has one.
func (s *S3) Put(ctx context.Context, object Object) error {
	input := &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(object.Key),
		Body:          bytes.NewReader(object.Body),
		ContentLength: aws.Int64(int64(len(object.Body))),
	}

	if object.ContentType != "" {
		input.ContentType = aws.String(object.ContentType)
	}

	if object.CacheControl != "" {
		input.CacheControl = aws.String(object.CacheControl)
	}

	if object.SHA256 != "" {
		input.Metadata = map[string]string{shaMetadata: object.SHA256}
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return s.explain(err)
	}

	return nil
}

// explain turns the storage's errors into what to do about them. The SDK's own
// messages hold request ids and repeat the operation.
func (s *S3) explain(err error) error {
	var urlErr *url.Error
	if stderrors.As(err, &urlErr) {
		return fmt.Errorf("can't reach the storage: %w", urlErr.Err)
	}

	var code, message string

	var apiErr smithy.APIError
	if stderrors.As(err, &apiErr) {
		code, message = apiErr.ErrorCode(), apiErr.ErrorMessage()
	}

	status := statusCode(err)

	switch {
	case code == "NoSuchBucket":
		return fmt.Errorf("the bucket %q does not exist", s.bucket)
	case code == "InvalidAccessKeyId":
		return stderrors.New("the storage doesn't know the access key ID")
	case code == "SignatureDoesNotMatch":
		return stderrors.New("the secret access key doesn't match the access key ID")
	case code == "AccessDenied" || status == http.StatusForbidden:
		return fmt.Errorf("the storage denied access to the bucket %q; check the keys and what they may do", s.bucket)
	case code == "PermanentRedirect" || status == http.StatusMovedPermanently:
		return fmt.Errorf("the bucket %q is in another region", s.bucket)
	case code != "" && message != "":
		return fmt.Errorf("the storage answered %s: %s", code, message)
	case code != "":
		return fmt.Errorf("the storage answered %s", code)
	}

	return err
}

// statusCode returns the HTTP status of the storage's answer, or 0 without one.
func statusCode(err error) int {
	var responseErr *smithyhttp.ResponseError
	if stderrors.As(err, &responseErr) {
		return responseErr.HTTPStatusCode()
	}

	return 0
}

// hasCode tells whether err is an S3 error with the code.
func hasCode(err error, code string) bool {
	var apiErr smithy.APIError

	return stderrors.As(err, &apiErr) && apiErr.ErrorCode() == code
}
