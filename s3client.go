package s3resource

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/aws/credentials"
	"github.com/awslabs/aws-sdk-go/service/s3"
	"github.com/concourse/s3gof3r"
)

//go:generate counterfeiter . S3Client

type S3Client interface {
	BucketFiles(bucketName string, prefixHint string) ([]string, error)
	BucketFileVersions(bucketName string, remotePath string) ([]string, error)

	UploadFile(bucketName string, remotePath string, localPath string) (string, error)
	DownloadFile(bucketName string, remotePath string, localPath string) error

	DeleteFile(bucketName string, remotePath string) error
	DeleteVersionedFile(bucketName string, remotePath string, versionID string) error

	URL(bucketName string, remotePath string, private bool, versionID string) string
}

type s3client struct {
	client         *s3.S3
	gopherClient   *s3gof3r.S3
	gopherMd5Check bool
}

func NewS3Client(accessKey string, secretKey string, regionName string, endpoint string, md5Check bool) (S3Client, error) {

	creds := credentials.NewStaticCredentials(accessKey, secretKey, "")

	authGopher := s3gof3r.Keys{
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	if len(regionName) == 0 {
		regionName = "us-east-1"
	}

	client := s3.New(&aws.Config{
		Region:      regionName,
		Credentials: creds,
	})

	gopherClient := s3gof3r.New("", authGopher)

	if len(endpoint) != 0 {
		endpointURL := fmt.Sprintf("https://%s", endpoint)
		client = s3.New(&aws.Config{
			Region:      regionName,
			Credentials: creds,
			Endpoint:    endpointURL,
		})
		gopherClient = s3gof3r.New(endpoint, authGopher)
	}

	return &s3client{
		client:         client,
		gopherClient:   gopherClient,
		gopherMd5Check: md5Check,
	}, nil
}

func (client *s3client) BucketFiles(bucketName string, prefixHint string) ([]string, error) {
	entries, err := client.getBucketContents(bucketName, prefixHint)

	if err != nil {
		return []string{}, err
	}

	paths := make([]string, 0, len(entries))

	for _, entry := range entries {
		paths = append(paths, *entry.Key)
	}
	return paths, nil
}

func (client *s3client) getBucketContents(bucketName string, prefix string) (map[string]*s3.Object, error) {
	bucketContents := map[string]*s3.Object{}
	marker := ""
	for {
		listObjectsResponse, err := client.client.ListObjects(&s3.ListObjectsInput{
			Bucket: aws.String(bucketName),
			Prefix: aws.String(prefix),
			Marker: aws.String(marker),
		})

		if err != nil {
			return bucketContents, err
		}

		lastKey := ""

		for _, key := range listObjectsResponse.Contents {
			bucketContents[*key.Key] = key

			lastKey = *key.Key
		}

		if *listObjectsResponse.IsTruncated {
			marker = *listObjectsResponse.Marker
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

	return bucketContents, nil
}

func (client *s3client) getBucketVersioning(bucketName string) (bool, error) {
	params := &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	}

	resp, err := client.client.GetBucketVersioning(params)
	if err != nil {
		return false, err
	}

	if resp.Status == nil {
		return false, nil
	}

	return *resp.Status == "Enabled", nil
}

func (client *s3client) BucketFileVersions(bucketName string, remotePath string) ([]string, error) {
	isBucketVersioned, err := client.getBucketVersioning(bucketName)
	if err != nil {
		return []string{}, err
	}

	if !isBucketVersioned {
		return []string{}, errors.New("bucket is not versioned")
	}

	bucketFiles, err := client.getVersionedBucketContents(bucketName, remotePath)

	if err != nil {
		return []string{}, err
	}

	versions := make([]string, 0, len(bucketFiles))

	for _, objectVersion := range bucketFiles[remotePath] {
		versions = append(versions, *objectVersion.VersionID)
	}

	return versions, nil
}

func (client *s3client) getVersionedBucketContents(bucketName string, prefix string) (map[string][]*s3.ObjectVersion, error) {
	versionedBucketContents := map[string][]*s3.ObjectVersion{}
	keyMarker := ""
	versionMarker := ""
	for {

		params := &s3.ListObjectVersionsInput{
			Bucket:    aws.String(bucketName),
			KeyMarker: aws.String(keyMarker),
			Prefix:    aws.String(prefix),
		}

		if versionMarker != "" {
			params.VersionIDMarker = aws.String(versionMarker)
		}

		listObjectVersionsResponse, err := client.client.ListObjectVersions(params)
		if err != nil {
			return versionedBucketContents, err
		}

		lastKey := ""
		lastVersionKey := ""

		for _, objectVersion := range listObjectVersionsResponse.Versions {
			versionedBucketContents[*objectVersion.Key] = append(versionedBucketContents[*objectVersion.Key], objectVersion)

			lastKey = *objectVersion.Key
			lastVersionKey = *objectVersion.VersionID
		}

		if *listObjectVersionsResponse.IsTruncated {
			keyMarker = *listObjectVersionsResponse.NextKeyMarker
			versionMarker = *listObjectVersionsResponse.NextVersionIDMarker
			if keyMarker == "" {
				// From the s3 docs: If response does not include the
				// NextMarker and it is truncated, you can use the value of the
				// last Key in the response as the marker in the subsequent
				// request to get the next set of object keys.
				keyMarker = lastKey
			}

			if versionMarker == "" {
				versionMarker = lastVersionKey
			}
		} else {
			break
		}

	}

	return versionedBucketContents, nil
}

func (client *s3client) UploadFile(bucketName string, remotePath string, localPath string) (string, error) {
	bucket := client.gopherClient.Bucket(bucketName)
	bucket.Config.Md5Check = client.gopherMd5Check

	localFile, err := os.Open(localPath)
	if err != nil {
		return "", err
	}

	remoteFile, err := bucket.PutWriter(remotePath, nil, nil)
	if err != nil {
		return "", err
	}

	if _, err = io.Copy(remoteFile, localFile); err != nil {
		return "", err
	}

	if err = remoteFile.Close(); err != nil {
		return "", err
	}

	if err = localFile.Close(); err != nil {
		return "", err
	}

	return remoteFile.VersionID, nil
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

func (client *s3client) URL(bucketName string, remotePath string, private bool, versionID string) string {
	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	}

	if versionID != "" {
		getObjectInput.VersionID = aws.String(versionID)
	}

	awsRequest, _ := client.client.GetObjectRequest(getObjectInput)

	var url string

	if private {
		url, _ = awsRequest.Presign(24 * time.Hour)
	} else {
		awsRequest.Build()
		url = awsRequest.HTTPRequest.URL.String()
	}

	return url
}

func (client *s3client) DeleteVersionedFile(bucketName string, remotePath string, versionID string) error {
	_, err := client.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket:    aws.String(bucketName),
		Key:       aws.String(remotePath),
		VersionID: aws.String(versionID),
	})

	return err
}

func (client *s3client) DeleteFile(bucketName string, remotePath string) error {
	_, err := client.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	})

	return err
}
