package s3resource

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
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

	URL(bucketName string, remotePath string, private bool, versionID string) (string, error)
}

// 12 retries works out to ~5 mins of total backoff time, though AWS randomizes
// the backoff to some extent so it may be as low as 4 or as high as 8 minutes
const MaxRetries = 12

type s3client struct {
	client         *s3.Client
	progressOutput io.Writer
}

type UploadFileOptions struct {
	Acl                  string
	ServerSideEncryption string
	KmsKeyId             string
	ContentType          string
	DisableMultipart     bool
	ChecksumAlgorithm    string
}

func NewUploadFileOptions() UploadFileOptions {
	return UploadFileOptions{
		Acl: "private",
	}
}

func NewS3Client(
	progressOutput io.Writer,
	awsConfig *aws.Config,
	endpoint string,
	disableSSL, usePathStyle, skipS3Checksums bool,
	checksumAlgorithm string,
) (S3Client, error) {
	s3Opts := []func(*s3.Options){}

	if endpoint != "" {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("error parsing given endpoint: %w", err)
		}
		if u.Scheme == "" {
			// source.Endpoint is a hostname with no Scheme
			scheme := "https://"
			if disableSSL {
				scheme = "http://"
			}
			endpoint = scheme + endpoint
		}

		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = &endpoint
			o.UsePathStyle = usePathStyle
			o.DisableLogOutputChecksumValidationSkipped = true
			if skipS3Checksums {
				o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
				o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
			}
			if checksumAlgorithm != "" {
				o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenSupported
			}
		})
	}

	client := s3.NewFromConfig(*awsConfig, s3Opts...)

	return &s3client{
		client:         client,
		progressOutput: progressOutput,
	}, nil
}

func NewAwsConfig(
	accessKey string,
	secretKey string,
	sessionToken string,
	roleToAssume string,
	regionName string,
	skipSSLVerification bool,
	caBundle string,
	useAwsCredsProvider bool,
) (*aws.Config, error) {
	var creds aws.CredentialsProvider

	if roleToAssume == "" && !useAwsCredsProvider {
		creds = aws.AnonymousCredentials{}
	}

	if accessKey != "" && secretKey != "" {
		creds = aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(accessKey, secretKey, sessionToken))
		_, err := creds.Retrieve(context.Background())
		if err != nil {
			return nil, err
		}
	}

	if len(regionName) == 0 {
		regionName = "us-east-1"
	}

	httpClient := awshttp.NewBuildableClient()
	if skipSSLVerification {
		httpClient = httpClient.WithTransportOptions(func(tr *http.Transport) {
			if tr.TLSClientConfig == nil {
				tr.TLSClientConfig = &tls.Config{}
			}
			tr.TLSClientConfig.InsecureSkipVerify = true
		})
	}
	if caBundle != "" {
		var caErr error
		httpClient = httpClient.WithTransportOptions(func(tr *http.Transport) {
			if tr.TLSClientConfig == nil {
				tr.TLSClientConfig = &tls.Config{}
			}
			if tr.TLSClientConfig.RootCAs == nil {
				tr.TLSClientConfig.RootCAs = x509.NewCertPool()
			}
			if !tr.TLSClientConfig.RootCAs.AppendCertsFromPEM([]byte(caBundle)) {
				caErr = fmt.Errorf("failed to load custom CA bundle PEM file")
			}
		})
		if caErr != nil {
			return nil, caErr
		}
	}

	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(regionName),
		config.WithHTTPClient(httpClient),
		config.WithRetryMaxAttempts(MaxRetries),
		config.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("error loading default AWS config: %w", err)
	}

	if roleToAssume != "" {
		stsClient := sts.NewFromConfig(cfg)
		stsCreds := stscreds.NewAssumeRoleProvider(stsClient, roleToAssume)
		roleCreds, err := stsCreds.Retrieve(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("error assuming role: %w", err)
		}

		cfg.Credentials = aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(
			roleCreds.AccessKeyID,
			roleCreds.SecretAccessKey,
			roleCreds.SessionToken,
		))
	}

	return &cfg, nil
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
	response, err := client.client.ListObjectsV2(context.TODO(), params)
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
	uploader := manager.NewUploader(client.client)

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
		if fSize <= manager.MinUploadPartSize {
			uploader.PartSize = manager.MinUploadPartSize
		}
	}

	progress := client.newProgressBar(fSize)
	defer progress.Wait()

	uploadInput := &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
		Body:   progress.ProxyReader(localFile),
		ACL:    types.ObjectCannedACL(options.Acl),
	}
	if options.ServerSideEncryption != "" {
		uploadInput.ServerSideEncryption = types.ServerSideEncryption(options.ServerSideEncryption)
	}
	if options.KmsKeyId != "" {
		uploadInput.SSEKMSKeyId = aws.String(options.KmsKeyId)
	}
	if options.ContentType != "" {
		uploadInput.ContentType = aws.String(options.ContentType)
	}
	if options.ChecksumAlgorithm != "" {
		uploadInput.ChecksumAlgorithm = types.ChecksumAlgorithm(options.ChecksumAlgorithm)
	}

	uploadOutput, err := uploader.Upload(context.TODO(), uploadInput)
	if err != nil {
		return "", err
	}

	// Have to manually complete the progress bar for empty files
	// See https://github.com/vbauerster/mpb/issues/7
	if fSize == 0 {
		progress.SetTotal(-1, true)
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

	object, err := client.client.HeadObject(context.TODO(), headObject)
	if err != nil {
		return err
	}

	progress := client.newProgressBar(*object.ContentLength)
	defer progress.Wait()

	downloader := manager.NewDownloader(client.client)

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

	_, err = downloader.Download(context.TODO(), progressWriterAt{localFile, progress.ProxyWriter(io.Discard)}, getObject)
	if err != nil {
		return err
	}

	// Have to manually complete the progress bar for empty files
	// See https://github.com/vbauerster/mpb/issues/7
	if *object.ContentLength == 0 {
		progress.SetTotal(-1, true)
	}

	return nil
}

func (client *s3client) SetTags(bucketName string, remotePath string, versionID string, tags map[string]string) error {
	var tagSet []types.Tag
	for key, value := range tags {
		tagSet = append(tagSet, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	putObjectTagging := &s3.PutObjectTaggingInput{
		Bucket:  aws.String(bucketName),
		Key:     aws.String(remotePath),
		Tagging: &types.Tagging{TagSet: tagSet},
	}
	if versionID != "" {
		putObjectTagging.VersionId = aws.String(versionID)
	}

	_, err := client.client.PutObjectTagging(context.TODO(), putObjectTagging)
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

	objectTagging, err := client.client.GetObjectTagging(context.TODO(), getObjectTagging)
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

func (client *s3client) URL(bucketName string, remotePath string, private bool, versionID string) (string, error) {
	if !private {
		var endpoint *string
		clientOptions := client.client.Options()

		if clientOptions.BaseEndpoint != nil {
			endpoint = clientOptions.BaseEndpoint
		}

		if endpoint == nil {
			endpoint = aws.String(fmt.Sprintf("https://s3.%s.amazonaws.com", clientOptions.Region))
		}

		// ResolveEndpoint() will return a URL with only the scheme and host
		// (e.g. https://bucket-name.s3.us-west-2.amazonaws.com). It will not
		// include the key/remotePath if you provide it.
		url, err := client.client.Options().EndpointResolverV2.ResolveEndpoint(
			context.Background(),
			s3.EndpointParameters{
				Endpoint: endpoint,
				Bucket:   &bucketName,
				Region:   &clientOptions.Region, //Not used to make the final URL string but is required
			})

		if err != nil {
			return "", fmt.Errorf("error resolving endpoint: %w", err)
		}

		return fmt.Sprintf("%s/%s", url.URI.String(), remotePath), nil
	}

	getObjectInput := &s3.GetObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	}

	if versionID != "" {
		getObjectInput.VersionId = aws.String(versionID)
	}

	presign := s3.NewPresignClient(client.client)
	request, err := presign.PresignGetObject(context.TODO(), getObjectInput, func(po *s3.PresignOptions) {
		po.Expires = 24 * time.Hour
	})

	if err != nil {
		return "", err
	}

	return request.URL, nil
}

func (client *s3client) DeleteVersionedFile(bucketName string, remotePath string, versionID string) error {
	_, err := client.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket:    aws.String(bucketName),
		Key:       aws.String(remotePath),
		VersionId: aws.String(versionID),
	})

	return err
}

func (client *s3client) DeleteFile(bucketName string, remotePath string) error {
	_, err := client.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(remotePath),
	})

	return err
}

func (client *s3client) getBucketVersioning(bucketName string) (bool, error) {
	params := &s3.GetBucketVersioningInput{
		Bucket: aws.String(bucketName),
	}

	resp, err := client.client.GetBucketVersioning(context.TODO(), params)
	if err != nil {
		return false, err
	}

	return resp.Status == types.BucketVersioningStatusEnabled, nil
}

func (client *s3client) getVersionedBucketContents(bucketName string, prefix string) (map[string][]types.ObjectVersion, error) {
	versionedBucketContents := map[string][]types.ObjectVersion{}
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

		listObjectVersionsResponse, err := client.client.ListObjectVersions(context.TODO(), params)
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

func (client *s3client) newProgressBar(total int64) *mpb.Bar {
	pg := mpb.New(mpb.WithWidth(80), mpb.WithOutput(client.progressOutput), mpb.WithAutoRefresh())
	bar := pg.New(total, mpb.BarStyle(),
		mpb.PrependDecorators(
			decor.Counters(decor.SizeB1024(0), "% .2f / % .2f"),
		),
		mpb.AppendDecorators(
			decor.NewPercentage("%d - "),
			decor.AverageSpeed(decor.SizeB1024(0), "% .2f"),
		),
	)
	return bar
}

func (client *s3client) isGCSHost() bool {
	return (client.client.Options().BaseEndpoint != nil && strings.Contains(*client.client.Options().BaseEndpoint, "storage.googleapis.com"))
}
