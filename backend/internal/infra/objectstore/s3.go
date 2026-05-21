package objectstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	platformtracing "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/observability/tracing"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type S3Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	ForcePathStyle  bool
}

type S3Store struct {
	client *s3.Client
	bucket string
	prefix string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3Store, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("s3 bucket is required")
	}
	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "auto"
	}
	options := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}
	if strings.TrimSpace(cfg.AccessKeyID) != "" || strings.TrimSpace(cfg.SecretAccessKey) != "" {
		options = append(options, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			strings.TrimSpace(cfg.AccessKeyID),
			strings.TrimSpace(cfg.SecretAccessKey),
			"",
		)))
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = cfg.ForcePathStyle
	})
	return &S3Store{client: client, bucket: bucket, prefix: normalizeKey(cfg.Prefix)}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, body io.Reader, opts PutOptions) (ObjectInfo, error) {
	normalizedKey := normalizeKey(key)
	if normalizedKey == "" {
		return ObjectInfo{}, ErrInvalidKey
	}
	objectKey := s.objectKey(normalizedKey)
	input := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
		Body:   body,
	}
	if opts.SizeBytes >= 0 {
		input.ContentLength = aws.Int64(opts.SizeBytes)
	}
	if contentType := strings.TrimSpace(opts.ContentType); contentType != "" {
		input.ContentType = aws.String(contentType)
	}
	ctx, span := s.startSpan(ctx, "objectstore.s3.put",
		attribute.Int64("objectstore.size_bytes", opts.SizeBytes),
		attribute.String("objectstore.content_type", strings.TrimSpace(opts.ContentType)),
	)
	_, err := s.client.PutObject(ctx, input)
	platformtracing.RecordError(span, err)
	span.End()
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: normalizedKey, SizeBytes: opts.SizeBytes, ContentType: strings.TrimSpace(opts.ContentType), ModTime: time.Now()}, nil
}

func (s *S3Store) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	normalizedKey := normalizeKey(key)
	if normalizedKey == "" {
		return nil, ObjectInfo{}, ErrInvalidKey
	}
	objectKey := s.objectKey(normalizedKey)
	ctx, span := s.startSpan(ctx, "objectstore.s3.open")
	output, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		platformtracing.RecordError(span, err)
		span.End()
		if isS3NotFound(err) {
			return nil, ObjectInfo{}, ErrNotFound
		}
		return nil, ObjectInfo{}, err
	}
	info := ObjectInfo{Key: normalizedKey}
	if output.ContentLength != nil {
		info.SizeBytes = *output.ContentLength
	}
	if output.ContentType != nil {
		info.ContentType = *output.ContentType
	}
	if output.LastModified != nil {
		info.ModTime = *output.LastModified
	}
	span.SetAttributes(attribute.Int64("objectstore.size_bytes", info.SizeBytes))
	return &tracedReadCloser{ReadCloser: output.Body, span: span}, info, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	normalizedKey := normalizeKey(key)
	if normalizedKey == "" {
		return nil
	}
	objectKey := s.objectKey(normalizedKey)
	ctx, span := s.startSpan(ctx, "objectstore.s3.delete")
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectKey),
	})
	platformtracing.RecordError(span, err)
	span.End()
	return err
}

func (s *S3Store) startSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	baseAttrs := []attribute.KeyValue{
		attribute.String("objectstore.system", "s3"),
		attribute.String("objectstore.bucket", s.bucket),
	}
	baseAttrs = append(baseAttrs, attrs...)
	return platformtracing.Start(ctx, name, trace.WithSpanKind(trace.SpanKindClient), trace.WithAttributes(baseAttrs...))
}

type tracedReadCloser struct {
	io.ReadCloser
	span     trace.Span
	once     sync.Once
	closeErr error
}

func (r *tracedReadCloser) Read(p []byte) (int, error) {
	n, err := r.ReadCloser.Read(p)
	if err != nil && !errors.Is(err, io.EOF) {
		platformtracing.RecordError(r.span, err)
	}
	return n, err
}

func (r *tracedReadCloser) Close() error {
	r.once.Do(func() {
		r.closeErr = r.ReadCloser.Close()
		if r.closeErr != nil {
			platformtracing.RecordError(r.span, r.closeErr)
		}
		r.span.End()
	})
	return r.closeErr
}

func (s *S3Store) objectKey(key string) string {
	normalizedKey := normalizeKey(key)
	if s == nil || s.prefix == "" {
		return normalizedKey
	}
	return s.prefix + "/" + normalizedKey
}

func (s *S3Store) Materialize(ctx context.Context, key string) (string, func(), error) {
	reader, _, err := s.Open(ctx, key)
	if err != nil {
		return "", nil, err
	}
	defer reader.Close() //nolint:errcheck
	file, err := os.CreateTemp("", "deeix-chat-object-*")
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }
	if _, err = io.Copy(file, reader); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err = file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

func isS3NotFound(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return code == "NoSuchKey" || code == "NotFound" || code == "404"
	}
	return false
}
