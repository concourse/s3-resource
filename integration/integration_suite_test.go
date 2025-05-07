package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	s3resource "github.com/concourse/s3-resource"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
}

var (
	accessKeyID         = os.Getenv("S3_TESTING_ACCESS_KEY_ID")
	secretAccessKey     = os.Getenv("S3_TESTING_SECRET_ACCESS_KEY")
	sessionToken        = os.Getenv("S3_TESTING_SESSION_TOKEN")
	awsRoleARN          = os.Getenv("S3_TESTING_AWS_ROLE_ARN")
	versionedBucketName = os.Getenv("S3_VERSIONED_TESTING_BUCKET")
	bucketName          = os.Getenv("S3_TESTING_BUCKET")
	regionName          = os.Getenv("S3_TESTING_REGION")
	endpoint            = os.Getenv("S3_ENDPOINT")
	v2signing           = os.Getenv("S3_V2_SIGNING")
	pathStyle           = len(os.Getenv("S3_USE_PATH_STYLE")) > 0
	awsConfig           *aws.Config
	s3client            s3resource.S3Client
	s3Service           *s3.Client

	checkPath string
	inPath    string
	outPath   string
)

type suiteData struct {
	CheckPath string
	InPath    string
	OutPath   string
}

func findOrCreate(binName string) string {
	resourcePath := "/opt/resource/" + binName
	if _, err := os.Stat(resourcePath); err == nil {
		return resourcePath
	} else {
		path, err := gexec.Build("github.com/concourse/s3-resource/cmd/" + binName)
		Ω(err).ShouldNot(HaveOccurred())
		return path
	}
}

func getSessionTokenS3Client(awsConfig *aws.Config) (*s3.Client, s3resource.S3Client) {
	stsClient := sts.NewFromConfig(*awsConfig)

	duration := int32(900)
	params := &sts.GetSessionTokenInput{
		DurationSeconds: &duration,
	}

	resp, err := stsClient.GetSessionToken(context.TODO(), params)
	Ω(err).ShouldNot(HaveOccurred())

	newAwsConfig, err := s3resource.NewAwsConfig(
		*resp.Credentials.AccessKeyId,
		*resp.Credentials.SecretAccessKey,
		*resp.Credentials.SessionToken,
		awsRoleARN,
		regionName,
		false,
		false,
	)
	Ω(err).ShouldNot(HaveOccurred())
	s3client, err := s3resource.NewS3Client(
		io.Discard,
		newAwsConfig,
		endpoint,
		false,
		pathStyle,
	)
	Ω(err).ShouldNot(HaveOccurred())

	return s3Service, s3client
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cp := findOrCreate("check")
	ip := findOrCreate("in")
	op := findOrCreate("out")

	data, err := json.Marshal(suiteData{
		CheckPath: cp,
		InPath:    ip,
		OutPath:   op,
	})

	Ω(err).ShouldNot(HaveOccurred())

	return data
}, func(data []byte) {
	var sd suiteData
	err := json.Unmarshal(data, &sd)
	Ω(err).ShouldNot(HaveOccurred())

	checkPath = sd.CheckPath
	inPath = sd.InPath
	outPath = sd.OutPath

	if accessKeyID != "" {
		Ω(accessKeyID).ShouldNot(BeEmpty(), "must specify $S3_TESTING_ACCESS_KEY_ID")
		Ω(secretAccessKey).ShouldNot(BeEmpty(), "must specify $S3_TESTING_SECRET_ACCESS_KEY")
		Ω(versionedBucketName).ShouldNot(BeEmpty(), "must specify $S3_VERSIONED_TESTING_BUCKET")
		Ω(bucketName).ShouldNot(BeEmpty(), "must specify $S3_TESTING_BUCKET")
		Ω(regionName).ShouldNot(BeEmpty(), "must specify $S3_TESTING_REGION")
		Ω(endpoint).ShouldNot(BeEmpty(), "must specify $S3_ENDPOINT")

		awsConfig, err = s3resource.NewAwsConfig(
			accessKeyID,
			secretAccessKey,
			sessionToken,
			awsRoleARN,
			regionName,
			false,
			false,
		)
		Ω(err).ShouldNot(HaveOccurred())

		s3Service = s3.NewFromConfig(*awsConfig)
		s3client, err = s3resource.NewS3Client(
			io.Discard,
			awsConfig,
			endpoint,
			false,
			pathStyle,
		)
		Ω(err).ShouldNot(HaveOccurred())
	}
})

var _ = BeforeEach(func() {
	if s3client == nil {
		Skip("Environment variables need to be set for AWS integration")
	}
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

func TestIn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

func buildEndpoint(bucket string, endpoint string) string {
	if endpoint == "" {
		return "https://s3.amazonaws.com/" + bucket
	} else {
		return endpoint + "/" + bucket
	}
}
