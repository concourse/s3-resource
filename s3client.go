package s3resource

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/concourse/s3gof3r"
	"github.com/mitchellh/goamz/aws"
	"github.com/mitchellh/goamz/s3"
)

//go:generate counterfeiter . S3Client

type S3Client interface {
	BucketFiles(bucketName string, prefixHint string) ([]string, error)

	UploadFile(bucketName string, remotePath string, localPath string) error
	DownloadFile(bucketName string, remotePath string, localPath string) error

	URL(bucketName string, remotePath string, private bool) string
}

type s3client struct {
	client         *s3.S3
	gopherClient   *s3gof3r.S3
	gopherMd5Check bool
}

func NewS3Client(accessKey string, secretKey string, regionName string, endpoint string, md5Check bool) (S3Client, error) {
	auth := aws.Auth{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	authGopher := s3gof3r.Keys{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	if len(regionName) == 0 {
		regionName = aws.USEast.Name
	}

	region, ok := aws.Regions[regionName]
	if !ok {
		return nil, fmt.Errorf("No such region '%s'", regionName)
	}

	client := s3.New(auth, region)
	gopherClient := s3gof3r.New("", authGopher)

	if len(endpoint) != 0 {
		endpointURL := fmt.Sprintf("https://%s", endpoint)
		client = s3.New(auth, aws.Region{S3Endpoint: endpointURL})
		gopherClient = s3gof3r.New(endpoint, authGopher)
	}

	return &s3client{
		client:         client,
		gopherClient:   gopherClient,
		gopherMd5Check: md5Check,
	}, nil
}

func (client *s3client) BucketFiles(bucketName string, prefixHint string) ([]string, error) {
	bucket := client.client.Bucket(bucketName)
	entries, err := client.getBucketContents(bucket, prefixHint)
	if err != nil {
		return []string{}, err
	}

	paths := make([]string, 0, len(*entries))
	for entry := range *entries {
		paths = append(paths, entry)
	}

	return paths, nil
}

func (client *s3client) getBucketContents(bucket *s3.Bucket, prefix string) (*map[string]s3.Key, error) {
	bucketContents := map[string]s3.Key{}
	separator := ""
	marker := ""

	for {
		contents, err := bucket.List(prefix, separator, marker, 1000)
		if err != nil {
			return &bucketContents, err
		}

		lastKey := ""
		for _, key := range contents.Contents {
			bucketContents[key.Key] = key
			lastKey = key.Key
		}

		if contents.IsTruncated {
			marker = contents.NextMarker
			if marker == "" {
				// From the s3 docs: If response does not include the
				// NextMarker and it is truncated, you can use the value of the
				// last Key in the response as the marker in the subsequent
				// request to get the next set of object keys.
				marker = lastKey
			}
		} else {
			break
		}
	}

	return &bucketContents, nil
}

func (client *s3client) UploadFile(bucketName string, remotePath string, localPath string) error {
	bucket := client.gopherClient.Bucket(bucketName)
	bucket.Config.Md5Check = client.gopherMd5Check

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
	bucket := client.gopherClient.Bucket(bucketName)
	bucket.Config.Md5Check = client.gopherMd5Check

	remoteFile, _, err := bucket.GetReader(remotePath, nil)
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
