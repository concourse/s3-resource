package s3resource

import (
	"io"
	"os"

	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

type S3Client interface {
	BucketFiles(bucketName string) ([]string, error)

	DownloadFile(bucketName string, remotePath string, localPath string) error
}

type s3client struct {
	client *s3.S3
}

func NewS3Client(accessKey string, secretKey string) S3Client {
	auth := aws.Auth{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	return &s3client{
		client: s3.New(auth, aws.USEast),
	}
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
