package s3client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"github.com/TicketsBot/logarchiver/pkg/repository/model"
	"github.com/minio/minio-go/v7"
)

type S3Client struct {
	client     *minio.Client
	bucketName string
	bucket     model.Bucket
}

func NewS3Client(client *minio.Client, bucketName string) *S3Client {
	return &S3Client{
		client:     client,
		bucketName: bucketName,
	}
}

func (c *S3Client) GetTicket(ctx context.Context, guildId uint64, ticketId int) ([]byte, error) {
	fmt.Printf("GetTicket called with guildId: %d, ticketId: %d\n", guildId, ticketId)
	fmt.Printf("Context: %v\n", ctx)

	key := fmt.Sprintf("%d/%d", guildId, ticketId)
	fmt.Printf("Generated key: %s\n", key)

	// Step 1: Stat the object to get ETag
	info, err := c.client.StatObject(ctx, c.bucketName, key, minio.StatObjectOptions{})
	if err != nil {
		if isNotFoundErr(err) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}

	etag := info.ETag
	fmt.Printf("Original ETag: %s\n", etag)

	// Step 2: Strip "W/" prefix and quotes (if any)
	etag = strings.TrimPrefix(etag, "W/")
	etag = strings.Trim(etag, "\"")

	fmt.Printf("Cleaned ETag: %s\n", etag)

	// Step 3: Use If-Match with the cleaned ETag
	opts := minio.GetObjectOptions{}
	opts.SetMatchETag(etag)

	// Step 4: Get the object
	object, err := c.client.GetObject(ctx, c.bucketName, key, opts)
	if err != nil {
		if isNotFoundErr(err) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	defer object.Close()

	var buff bytes.Buffer
	if _, err := buff.ReadFrom(object); err != nil {
		return nil, err
	}

	return buff.Bytes(), nil
}


func (c *S3Client) StoreTicket(ctx context.Context, guildId uint64, ticketId int, data []byte) error {
	key := fmt.Sprintf("%d/%d", guildId, ticketId)

	_, err := c.client.PutObject(ctx, c.bucketName, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{
		ContentType:     "application/octet-stream",
		ContentEncoding: "zstd",
	})

	return err
}

func (c *S3Client) DeleteTicket(ctx context.Context, guildId uint64, ticketId int) error {
	key := fmt.Sprintf("%d/%d", guildId, ticketId)

	return c.client.RemoveObject(ctx, c.bucketName, key, minio.RemoveObjectOptions{})
}

// GetAllKeysForGuild returns all keys in the bucket for a given guild. This can be a very slow operation, and so
// is only recommended for use in manual scripts.
func (c *S3Client) GetAllKeysForGuild(ctx context.Context, guildId uint64) ([]string, error) {
	prefix := fmt.Sprintf("%d/", guildId)
	opts := minio.ListObjectsOptions{
		WithMetadata: true,
		Prefix:       prefix,
		Recursive:    true,
	}

	keys := make([]string, 0)
	for obj := range c.client.ListObjects(ctx, c.bucketName, opts) {
		keys = append(keys, obj.Key)
	}

	return keys, nil
}

// Minio returns the underlying minio client. This will be removed in the future, once the entries from the default
// bucket are migrated into the database.
func (c *S3Client) Minio() *minio.Client {
	return c.client
}

// BucketName returns the underlying minio client. This will be removed in the future, once the entries from the default
// bucket are migrated into the database.
func (c *S3Client) BucketName() string {
	return c.bucketName
}

func isNotFoundErr(err error) bool {
	var resp minio.ErrorResponse
	return errors.As(err, &resp) && resp.Code == "NoSuchKey"
}
