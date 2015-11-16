package integration_test

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/concourse/s3-resource"
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
var versionedBucketName = os.Getenv("S3_VERSIONED_TESTING_BUCKET")
var bucketName = os.Getenv("S3_TESTING_BUCKET")
var regionName = os.Getenv("S3_TESTING_REGION")
var endpoint = os.Getenv("S3_ENDPOINT")
var s3client s3resource.S3Client

var checkPath string
var inPath string
var outPath string

type suiteData struct {
	CheckPath string
	InPath    string
	OutPath   string
}

var _ = SynchronizedBeforeSuite(func() []byte {
	cp, err := gexec.Build("github.com/concourse/s3-resource/cmd/check")
	Ω(err).ShouldNot(HaveOccurred())
	ip, err := gexec.Build("github.com/concourse/s3-resource/cmd/in")
	Ω(err).ShouldNot(HaveOccurred())
	op, err := gexec.Build("github.com/concourse/s3-resource/cmd/out")
	Ω(err).ShouldNot(HaveOccurred())

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

	Ω(accessKeyID).ShouldNot(BeEmpty(), "must specify $S3_TESTING_ACCESS_KEY_ID")
	Ω(secretAccessKey).ShouldNot(BeEmpty(), "must specify $S3_TESTING_SECRET_ACCESS_KEY")
	Ω(versionedBucketName).ShouldNot(BeEmpty(), "must specify $S3_VERSIONED_TESTING_BUCKET")
	Ω(bucketName).ShouldNot(BeEmpty(), "must specify $S3_TESTING_BUCKET")
	Ω(regionName).ShouldNot(BeEmpty(), "must specify $S3_TESTING_REGION")

	s3client, err = s3resource.NewS3Client(
		ioutil.Discard,
		accessKeyID,
		secretAccessKey,
		regionName,
		endpoint,
	)
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
