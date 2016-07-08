package s3resource

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb"
)

//go:generate counterfeiter . S3Client

type S3Client interface {
	BucketFiles(bucketName string, prefixHint string) ([]string, error)
	BucketFileVersions(bucketName string, remotePath string) ([]string, error)

	UploadFile(bucketName string, remotePath string, localPath string, acl string) (string, error)
	DownloadFile(bucketName string, remotePath string, versionID string, localPath string) error

	DeleteFile(bucketName string, remotePath string) error
	DeleteVersionedFile(bucketName string, remotePath string, versionID string) error

	URL(bucketName string, remotePath string, private bool, versionID string) string
}

// 12 retries works out to ~5 mins of total backoff time, though AWS randomizes
// the backoff to some extent so it may be as low as 4 or as high as 8 minutes
const maxRetries = 12

type s3client struct {
	client  *s3.S3
	session *session.Session

	progressOutput io.Writer
}

func NewS3Client(
	progressOutput io.Writer,
	awsConfig *aws.Config,
) S3Client {
	sess := session.New(awsConfig)
	client := s3.New(sess, awsConfig)

	return &s3client{
		client:  client,
		session: sess,

		progressOutput: progressOutput,
	}
}

func NewAwsConfig(
	accessKey string,
	secretKey string,
	regionName string,
	endpoint string,
	disableSSL bool,
) *aws.Config {
	var creds *credentials.Credentials

	if accessKey == "" && secretKey == "" {
		creds = credentials.AnonymousCredentials
	} else {
		creds = credentials.NewStaticCredentials(accessKey, secretKey, "")
	}

	if len(regionName) == 0 {
		regionName = "us-east-1"
	}

	awsConfig := &aws.Config{
		Region:           aws.String(regionName),
		Credentials:      creds,
		S3ForcePathStyle: aws.Bool(true),
		MaxRetries:       aws.Int(maxRetries),
		DisableSSL:       aws.Bool(disableSSL),
	}

	if len(endpoint) != 0 {
		endpoint := fmt.Sprintf("%s", endpoint)
		awsConfig.Endpoint = &endpoint
	}

	return awsConfig
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
		versions = append(versions, *objectVersion.VersionId)
	}

	return versions, nil
}

func (client *s3client) UploadFile(bucketName string, remotePath string, localPath string, acl string) (string, error) {
	uploader := s3manager.NewUploader(client.session)

	stat, err := os.Stat(localPath)
	if err != nil {
		return "", err
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return "", err
	}

	defer localFile.Close()

	progress := client.newProgressBar(stat.Size())

	progress.Start()
	defer progress.Finish()

	uploadOutput, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
		Body:   progressSeekReaderAt{localFile, progress},
		ACL:    aws.String(acl),
	})
	if err != nil {
		return "", err
	}

	if uploadOutput.VersionID != nil {
		return *uploadOutput.VersionID, nil
	}

	return "", nil
}

func (client *s3client) DownloadFile(bucketName string, remotePath string, versionID string, localPath string) error {
	headObject := &s3.HeadObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	}

	if versionID != "" {
		headObject.VersionId = aws.String(versionID)
	}

	object, err := client.client.HeadObject(headObject)
	if err != nil {
		return err
	}

	progress := client.newProgressBar(*object.ContentLength)

	downloader := s3manager.NewDownloader(client.session)

	localFile, err := os.Create(localPath)
	if err != nil {
		return err
	}
	defer localFile.Close()

	getObject := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	}

	if versionID != "" {
		getObject.VersionId = aws.String(versionID)
	}

	progress.Start()
	defer progress.Finish()

	_, err = downloader.Download(progressWriterAt{localFile, progress}, getObject)
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
		getObjectInput.VersionId = aws.String(versionID)
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
		VersionId: aws.String(versionID),
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
			if listObjectsResponse.NextMarker == nil {
				// From the s3 docs: If response does not include the
				// NextMarker and it is truncated, you can use the value of the
				// last Key in the response as the marker in the subsequent
				// request to get the next set of object keys.
				marker = lastKey
			} else {
				marker = *listObjectsResponse.NextMarker
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
			params.VersionIdMarker = aws.String(versionMarker)
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
			lastVersionKey = *objectVersion.VersionId
		}

		if *listObjectVersionsResponse.IsTruncated {
			keyMarker = *listObjectVersionsResponse.NextKeyMarker
			versionMarker = *listObjectVersionsResponse.NextVersionIdMarker
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

func (client *s3client) newProgressBar(total int64) *pb.ProgressBar {
	progress := pb.New64(total)

	progress.Output = client.progressOutput
	progress.ShowSpeed = true
	progress.Units = pb.U_BYTES
	progress.NotPrint = true

	return progress.SetWidth(80)
}
