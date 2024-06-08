package integration_test

import (
	"encoding/json"
	"io"
	"os"

	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sts"
	s3resource "github.com/concourse/s3-resource"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
}

var accessKeyID = os.Getenv("S3_TESTING_ACCESS_KEY_ID")
var secretAccessKey = os.Getenv("S3_TESTING_SECRET_ACCESS_KEY")
var sessionToken = os.Getenv("S3_TESTING_SESSION_TOKEN")
var awsRoleARN = os.Getenv("S3_TESTING_AWS_ROLE_ARN")
var versionedBucketName = os.Getenv("S3_VERSIONED_TESTING_BUCKET")
var bucketName = os.Getenv("S3_TESTING_BUCKET")
var regionName = os.Getenv("S3_TESTING_REGION")
var endpoint = os.Getenv("S3_ENDPOINT")
var v2signing = os.Getenv("S3_V2_SIGNING")
var assumeRoleArn = ""
var awsConfig *aws.Config
var s3client s3resource.S3Client
var s3Service *s3.S3

var checkPath string
var inPath string
var outPath string

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

func getSessionTokenS3Client(awsConfig *aws.Config) (*s3.S3, s3resource.S3Client) {
	stsAwsConfig := &aws.Config{
		Region:      awsConfig.Region,
		Credentials: awsConfig.Credentials,
		MaxRetries:  awsConfig.MaxRetries,
		HTTPClient:  awsConfig.HTTPClient,
	}

	stsSess, err := session.NewSession(stsAwsConfig)
	Ω(err).ShouldNot(HaveOccurred())
	svc := sts.New(stsSess, stsAwsConfig)

	duration := int64(900)
	params := &sts.GetSessionTokenInput{
		DurationSeconds: &duration,
	}

	resp, err := svc.GetSessionToken(params)
	Ω(err).ShouldNot(HaveOccurred())

	newAwsConfig := s3resource.NewAwsConfig(
		*resp.Credentials.AccessKeyId,
		*resp.Credentials.SecretAccessKey,
		*resp.Credentials.SessionToken,
		assumeRoleArn,
		regionName,
		endpoint,
		false,
		false,
	)

	sess, err := session.NewSession(awsConfig)
	Ω(err).ShouldNot(HaveOccurred())
	s3Service = s3.New(sess, awsConfig, newAwsConfig)
	s3client, err = s3resource.NewS3Client(io.Discard, awsConfig, v2signing == "true", awsRoleARN)
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

		awsConfig = s3resource.NewAwsConfig(
			accessKeyID,
			secretAccessKey,
			sessionToken,
			assumeRoleArn,
			regionName,
			endpoint,
			false,
			false,
		)

		additionalAwsConfig := aws.Config{}
		if len(awsRoleARN) != 0 {
			stsConfig := awsConfig.Copy()
			stsConfig.Endpoint = nil
			stsSession := session.Must(session.NewSession(stsConfig))
			roleCredentials := stscreds.NewCredentials(stsSession, awsRoleARN)

			additionalAwsConfig.Credentials = roleCredentials
		}

		sess, err := session.NewSession(awsConfig)
		Ω(err).ShouldNot(HaveOccurred())
		s3Service = s3.New(sess, awsConfig, &additionalAwsConfig)
		s3client, err = s3resource.NewS3Client(io.Discard, awsConfig, v2signing == "true", awsRoleARN)
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
