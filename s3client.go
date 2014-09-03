package s3resource

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
	"github.com/rlmcpherson/s3gof3r"
)

type S3Client interface {
	BucketFiles(bucketName string) ([]string, error)

	UploadFile(bucketName string, remotePath string, localPath string) error
	DownloadFile(bucketName string, remotePath string, localPath string) error

	URL(bucketName string, remotePath string, private bool) string
}

type s3client struct {
	client       *s3.S3
	gopherClient *s3gof3r.S3
}

func NewS3Client(accessKey string, secretKey string, regionName string) (S3Client, error) {
	auth := aws.Auth{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	authGopher := s3gof3r.Keys{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	region, ok := aws.Regions[regionName]
	if !ok {
		return nil, errors.New(fmt.Sprintf("No such region '%s'", regionName))
	}

	return &s3client{
		client:       s3.New(auth, region),
		gopherClient: s3gof3r.New("", authGopher),
	}, nil
}

func (client *s3client) BucketFiles(bucketName string) ([]string, error) {
	bucket := client.client.Bucket(bucketName)
	entries, err := bucket.GetBucketContents()
	if err != nil {
		return []string{}, err
	}

	paths := make([]string, 0, len(*entries))
	for entry := range *entries {
		paths = append(paths, entry)
	}

	return paths, nil
}

func (client *s3client) UploadFile(bucketName string, remotePath string, localPath string) error {
	bucket := client.gopherClient.Bucket(bucketName)

	localFile, err := os.Open(localPath)
	if err != nil {
		return err
	}

	remoteFile, err := bucket.PutWriter(remotePath, nil, nil)
	if err != nil {
		return err
	}

	if _, err = io.Copy(remoteFile, localFile); err != nil {
		return err
	}

	if err = remoteFile.Close(); err != nil {
		return err
	}

	if err = localFile.Close(); err != nil {
		return err
	}

	return nil
}

func (client *s3client) DownloadFile(bucketName string, remotePath string, localPath string) error {
	bucket := client.client.Bucket(bucketName)

	remoteFile, err := bucket.GetReader(remotePath)
	if err != nil {
		return err
	}
	defer remoteFile.Close()

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return err
	}

	return nil
}

func (client *s3client) URL(bucketName string, remotePath string, private bool) string {
	bucket := client.client.Bucket(bucketName)

	if private {
		return bucket.SignedURL(remotePath, time.Now().Add(24*time.Hour))
	}

	return bucket.URL(remotePath)
}
