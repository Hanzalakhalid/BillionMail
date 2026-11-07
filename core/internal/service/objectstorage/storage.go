package objectstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

type Storage interface {
	Put(context.Context, string, io.Reader, string) error
	Get(context.Context, string) (io.ReadCloser, string, error)
	Delete(context.Context, string) error
	PresignGet(context.Context, string, time.Duration) (string, error)
	PresignPut(context.Context, string, string, time.Duration) (string, error)
	PresignUpload(context.Context, string, string, int64, time.Duration) (UploadForm, error)
	Health(context.Context) error
}

type UploadForm struct {
	URL    string            `json:"url"`
	Fields map[string]string `json:"fields"`
}

// PresignUpload constrains the exact object, declared MIME type, and byte size
// in the storage-verified POST policy. Bytes are still validated after upload.
func (s *S3Store) PresignUpload(ctx context.Context, key, contentType string, maxBytes int64, ttl time.Duration) (UploadForm, error) {
	if err := ValidateKey(key); err != nil {
		return UploadForm{}, err
	}
	if err := duration(ttl); err != nil {
		return UploadForm{}, err
	}
	if contentType == "" || maxBytes < 1 || maxBytes > 1024*1024*1024 {
		return UploadForm{}, errors.New("content type and upload limit up to 1 GiB are required")
	}
	out, err := s.presigner.PresignPostObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(o *s3.PresignPostOptions) {
		o.Expires = ttl
		o.Conditions = []interface{}{[]interface{}{"content-length-range", int64(1), maxBytes}, map[string]string{"Content-Type": contentType}}
	})
	if err != nil {
		return UploadForm{}, err
	}
	out.Values["Content-Type"] = contentType
	return UploadForm{URL: out.URL, Fields: out.Values}, nil
}

type Config struct {
	Bucket         string
	Region         string
	Endpoint       string
	PublicEndpoint string
	AccessKey      string
	SecretKey      string
	PathStyle      bool
}

func ConfigFromEnv() Config {
	return Config{Bucket: os.Getenv("MEDIA_BUCKET"), Region: os.Getenv("MEDIA_REGION"), Endpoint: os.Getenv("MEDIA_ENDPOINT"), PublicEndpoint: os.Getenv("MEDIA_PUBLIC_ENDPOINT"), AccessKey: os.Getenv("MEDIA_ACCESS_KEY"), SecretKey: os.Getenv("MEDIA_SECRET_KEY"), PathStyle: strings.EqualFold(os.Getenv("MEDIA_PATH_STYLE"), "true")}
}

func (c Config) Validate() error {
	if c.Bucket == "" || c.Region == "" {
		return errors.New("MEDIA_BUCKET and MEDIA_REGION are required")
	}
	if (c.AccessKey == "") != (c.SecretKey == "") {
		return errors.New("media access key and secret key must be configured together")
	}
	for _, endpoint := range []string{c.Endpoint, c.PublicEndpoint} {
		if endpoint == "" {
			continue
		}
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
			return errors.New("media endpoints must be HTTP origins without credentials, paths or query parameters")
		}
	}
	return nil
}

type S3Store struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	region    string
}

func New(ctx context.Context, c Config) (*S3Store, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	options := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(c.Region)}
	if c.AccessKey != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, "")))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("load media storage configuration: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = c.PathStyle
		if c.Endpoint != "" {
			o.BaseEndpoint = aws.String(c.Endpoint)
		}
	})
	signingClient := client
	if c.PublicEndpoint != "" {
		signingClient = s3.NewFromConfig(cfg, func(o *s3.Options) { o.UsePathStyle = c.PathStyle; o.BaseEndpoint = aws.String(c.PublicEndpoint) })
	}
	return &S3Store{client: client, presigner: s3.NewPresignClient(signingClient), bucket: c.Bucket, region: c.Region}, nil
}

func (s *S3Store) EnsureBucket(ctx context.Context) error {
	if err := s.Health(ctx); err == nil {
		return nil
	} else {
		var response interface{ HTTPStatusCode() int }
		if !errors.As(err, &response) || response.HTTPStatusCode() != 404 {
			return err
		}
	}
	input := &s3.CreateBucketInput{Bucket: aws.String(s.bucket)}
	if s.region != "us-east-1" {
		input.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{LocationConstraint: s3types.BucketLocationConstraint(s.region)}
	}
	_, err := s.client.CreateBucket(ctx, input)
	var owned *s3types.BucketAlreadyOwnedByYou
	if errors.As(err, &owned) {
		return nil
	}
	return err
}

// ValidateKey confines each asset to a workspace prefix. The caller still checks
// workspace authorization before constructing a storage operation.
func ValidateKey(key string) error {
	if key == "" || len(key) > 1024 || strings.HasPrefix(key, "/") || strings.Contains(key, "\\") || strings.Contains(key, "//") {
		return errors.New("invalid media object key")
	}
	parts := strings.Split(key, "/")
	if len(parts) < 3 || parts[0] != "companies" || parts[1] == "" {
		return errors.New("media key must start with companies/<company-id>/")
	}
	companyID, err := uuid.Parse(parts[1])
	if err != nil || companyID == uuid.Nil || companyID.String() != parts[1] {
		return errors.New("media key must use a canonical company UUID")
	}
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return errors.New("invalid media object key segment")
		}
	}
	return nil
}

func (s *S3Store) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if body == nil || contentType == "" {
		return errors.New("media body and content type are required")
	}
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: body, ContentType: aws.String(contentType)})
	return err
}
func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, string, error) {
	if err := ValidateKey(key); err != nil {
		return nil, "", err
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, "", err
	}
	return out.Body, aws.ToString(out.ContentType), nil
}
func (s *S3Store) Delete(ctx context.Context, key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}
func duration(d time.Duration) error {
	if d < time.Second || d > time.Hour {
		return errors.New("signed media URL lifetime must be between 1 second and 1 hour")
	}
	return nil
}
func (s *S3Store) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if err := duration(ttl); err != nil {
		return "", err
	}
	out, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
func (s *S3Store) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	if err := duration(ttl); err != nil {
		return "", err
	}
	if contentType == "" {
		return "", errors.New("content type is required")
	}
	out, err := s.presigner.PresignPutObject(ctx, &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ContentType: aws.String(contentType)}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
func (s *S3Store) Health(ctx context.Context) error {
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)})
	return err
}
