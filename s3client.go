package s3resource

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"

	"net/http"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/cheggaaa/pb/v3"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate -o fakes . S3Client
type S3Client interface {
	BucketFiles(bucketName string, prefixHint string) ([]string, error)
	BucketFileVersions(bucketName string, remotePath string) ([]string, error)

	ChunkedBucketList(bucketName string, prefix string, continuationToken *string) (BucketListChunk, error)

	UploadFile(bucketName string, remotePath string, localPath string, options UploadFileOptions) (string, error)
	DownloadFile(bucketName string, remotePath string, versionID string, localPath string) error

	SetTags(bucketName string, remotePath string, versionID string, tags map[string]string) error
	DownloadTags(bucketName string, remotePath string, versionID string, localPath string) error

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

type UploadFileOptions struct {
	Acl                  string
	ServerSideEncryption string
	KmsKeyId             string
	ContentType          string
	DisableMultipart     bool
}

func NewUploadFileOptions() UploadFileOptions {
	return UploadFileOptions{
		Acl: "private",
	}
}

func NewS3Client(
	progressOutput io.Writer,
	awsConfig *aws.Config,
	useV2Signing bool,
	roleToAssume string,
) S3Client {
	sess := session.Must(session.NewSession())

	assumedRoleAwsConfig := fetchCredentialsForRoleIfDefined(roleToAssume, awsConfig)

	client := s3.New(sess, awsConfig, &assumedRoleAwsConfig)

	if useV2Signing {
		setv2Handlers(client)
	}

	return &s3client{
		client:  client,
		session: sess,

		progressOutput: progressOutput,
	}
}

func fetchCredentialsForRoleIfDefined(roleToAssume string, awsConfig *aws.Config) aws.Config {
	assumedRoleAwsConfig := aws.Config{}
	if len(roleToAssume) != 0 {
		stsConfig := awsConfig.Copy()
		stsConfig.Endpoint = nil
		stsSession := session.Must(session.NewSession(stsConfig))
		roleCredentials := stscreds.NewCredentials(stsSession, roleToAssume)

		assumedRoleAwsConfig.Credentials = roleCredentials
	}
	return assumedRoleAwsConfig
}

func NewAwsConfig(
	accessKey string,
	secretKey string,
	sessionToken string,
	regionName string,
	endpoint string,
	disableSSL bool,
	skipSSLVerification bool,
) *aws.Config {
	var httpClient *http.Client
	if skipSSLVerification {
		httpClient = &http.Client{Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}}
	} else {
		httpClient = http.DefaultClient
	}

	awsConfig := &aws.Config{
		Region:                        aws.String(regionName),
		S3ForcePathStyle:              aws.Bool(true),
		MaxRetries:                    aws.Int(maxRetries),
		DisableSSL:                    aws.Bool(disableSSL),
		HTTPClient:                    httpClient,
		CredentialsChainVerboseErrors: aws.Bool(true),
	}

	if accessKey != "" && secretKey != "" {
		awsConfig.Credentials = credentials.NewStaticCredentials(accessKey, secretKey, sessionToken)
	} else {
		println("Using default credential chain for authentication.")
	}

	if len(regionName) == 0 {
		regionName = "us-east-1"
	}

	if len(endpoint) != 0 {
		endpoint := fmt.Sprintf("%s", endpoint)
		awsConfig.Endpoint = &endpoint
	}

	return awsConfig
}

// BucketFiles returns all the files in bucketName immediately under directoryPrefix
func (client *s3client) BucketFiles(bucketName string, directoryPrefix string) ([]string, error) {
	if !strings.HasSuffix(directoryPrefix, "/") {
		directoryPrefix = directoryPrefix + "/"
	}
	var (
		continuationToken *string
		truncated         bool
		paths             []string
	)
	for continuationToken, truncated = nil, true; truncated; {
		s3ListChunk, err := client.ChunkedBucketList(bucketName, directoryPrefix, continuationToken)
		if err != nil {
			return []string{}, err
		}
		truncated = s3ListChunk.Truncated
		continuationToken = s3ListChunk.ContinuationToken
		paths = append(paths, s3ListChunk.Paths...)
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

type BucketListChunk struct {
	Truncated         bool
	ContinuationToken *string
	CommonPrefixes    []string
	Paths             []string
}

// ChunkedBucketList lists the S3 bucket `bucketName` content's under `prefix` one chunk at a time
//
// The returned `BucketListChunk` contains part of the files and subdirectories
// present in `bucketName` under `prefix`. The files are listed in `Paths` and
// the subdirectories in `CommonPrefixes`. If the returned chunk does not
// include all the files and subdirectories, the `Truncated` flag will be set
// to `true` and the `ContinuationToken` can be used to retrieve the next chunk.
func (client *s3client) ChunkedBucketList(bucketName string, prefix string, continuationToken *string) (BucketListChunk, error) {
	params := &s3.ListObjectsV2Input{
		Bucket:            aws.String(bucketName),
		ContinuationToken: continuationToken,
		Delimiter:         aws.String("/"),
		Prefix:            aws.String(prefix),
	}
	response, err := client.client.ListObjectsV2(params)
	if err != nil {
		return BucketListChunk{}, err
	}
	commonPrefixes := make([]string, 0, len(response.CommonPrefixes))
	paths := make([]string, 0, len(response.Contents))
	for _, commonPrefix := range response.CommonPrefixes {
		commonPrefixes = append(commonPrefixes, *commonPrefix.Prefix)
	}
	for _, path := range response.Contents {
		paths = append(paths, *path.Key)
	}
	return BucketListChunk{
		Truncated:         *response.IsTruncated,
		ContinuationToken: response.NextContinuationToken,
		CommonPrefixes:    commonPrefixes,
		Paths:             paths,
	}, nil
}

func (client *s3client) UploadFile(bucketName string, remotePath string, localPath string, options UploadFileOptions) (string, error) {
	uploader := s3manager.NewUploaderWithClient(client.client)

	if client.isGCSHost() {
		// GCS returns `InvalidArgument` on multipart uploads
		uploader.MaxUploadParts = 1
	}

	stat, err := os.Stat(localPath)
	if err != nil {
		return "", err
	}

	localFile, err := os.Open(localPath)
	if err != nil {
		return "", err
	}

	defer localFile.Close()

	// Automatically adjust partsize for larger files.
	fSize := stat.Size()
	if !options.DisableMultipart {
		if fSize > int64(uploader.MaxUploadParts)*uploader.PartSize {
			partSize := fSize / int64(uploader.MaxUploadParts)
			if fSize%int64(uploader.MaxUploadParts) != 0 {
				partSize++
			}
			uploader.PartSize = partSize
		}
	} else {
		uploader.MaxUploadParts = 1
		uploader.Concurrency = 1
		uploader.PartSize = fSize + 1
		if fSize <= s3manager.MinUploadPartSize {
			uploader.PartSize = s3manager.MinUploadPartSize
		}
	}

	progress := client.newProgressBar(fSize)

	progress.Start()
	defer progress.Finish()

	uploadInput := s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
		Body:   progressReader{localFile, progress},
		ACL:    aws.String(options.Acl),
	}
	if options.ServerSideEncryption != "" {
		uploadInput.ServerSideEncryption = aws.String(options.ServerSideEncryption)
	}
	if options.KmsKeyId != "" {
		uploadInput.SSEKMSKeyId = aws.String(options.KmsKeyId)
	}
	if options.ContentType != "" {
		uploadInput.ContentType = aws.String(options.ContentType)
	}

	uploadOutput, err := uploader.Upload(&uploadInput)
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

	downloader := s3manager.NewDownloaderWithClient(client.client)

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

func (client *s3client) SetTags(bucketName string, remotePath string, versionID string, tags map[string]string) error {
	var tagSet []*s3.Tag
	for key, value := range tags {
		tagSet = append(tagSet, &s3.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	putObjectTagging := &s3.PutObjectTaggingInput{
		Bucket:  aws.String(bucketName),
		Key:     aws.String(remotePath),
		Tagging: &s3.Tagging{TagSet: tagSet},
	}
	if versionID != "" {
		putObjectTagging.VersionId = aws.String(versionID)
	}

	_, err := client.client.PutObjectTagging(putObjectTagging)
	return err
}

func (client *s3client) DownloadTags(bucketName string, remotePath string, versionID string, localPath string) error {
	getObjectTagging := &s3.GetObjectTaggingInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	}
	if versionID != "" {
		getObjectTagging.VersionId = aws.String(versionID)
	}

	objectTagging, err := client.client.GetObjectTagging(getObjectTagging)
	if err != nil {
		return err
	}

	tags := map[string]string{}
	for _, tag := range objectTagging.TagSet {
		tags[*tag.Key] = *tag.Value
	}

	tagsJSON, err := json.Marshal(tags)
	if err != nil {
		return err
	}

	return os.WriteFile(localPath, tagsJSON, 0644)
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
			Bucket: aws.String(bucketName),
			Prefix: aws.String(prefix),
		}

		if keyMarker != "" {
			params.KeyMarker = aws.String(keyMarker)
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
	progress.SetWriter(client.progressOutput)
	return progress.Set(pb.Bytes, true)
}

func (client *s3client) isGCSHost() bool {
	return (client.session.Config.Endpoint != nil && strings.Contains(*client.session.Config.Endpoint, "storage.googleapis.com"))
}
