package r2

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Client struct {
	bucket  string
	s3      *s3.Client
	presign *s3.PresignClient
}

func New(accountID, accessKeyID, secretKey, bucket string) (*Client, error) {
	if accountID == "" || accessKeyID == "" || secretKey == "" || bucket == "" {
		return nil, fmt.Errorf("R2 is not configured")
	}
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String("https://" + accountID + ".r2.cloudflarestorage.com")
		o.UsePathStyle = true
	})
	return &Client{bucket: bucket, s3: client, presign: s3.NewPresignClient(client)}, nil
}

func (c *Client) PresignUpload(ctx context.Context, key, contentType string) (string, error) {
	result, err := c.presign.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key), ContentType: aws.String(contentType),
	}, func(o *s3.PresignOptions) { o.Expires = 10 * time.Minute })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (c *Client) PresignDownload(ctx context.Context, key string) (string, error) {
	result, err := c.presign.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.bucket), Key: aws.String(key),
	}, func(o *s3.PresignOptions) { o.Expires = 5 * time.Minute })
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func (c *Client) Stat(ctx context.Context, key string) (int64, string, error) {
	result, err := c.s3.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	if err != nil {
		return 0, "", err
	}
	return aws.ToInt64(result.ContentLength), aws.ToString(result.ContentType), nil
}

func (c *Client) Delete(ctx context.Context, key string) error {
	_, err := c.s3.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(c.bucket), Key: aws.String(key)})
	return err
}
