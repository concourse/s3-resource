package integration_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/out"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("out", func() {
	var (
		command   *exec.Cmd
		stdin     *bytes.Buffer
		session   *gexec.Session
		sourceDir string

		expectedExitStatus int
	)

	BeforeEach(func() {
		var err error
		sourceDir, err = ioutil.TempDir("", "s3_out_integration_test")
		Ω(err).ShouldNot(HaveOccurred())

		stdin = &bytes.Buffer{}
		expectedExitStatus = 0

		command = exec.Command(outPath, sourceDir)
		command.Stdin = stdin
	})

	AfterEach(func() {
		err := os.RemoveAll(sourceDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 10*time.Second).Should(gexec.Exit(expectedExitStatus))
	})

	Context("with a non-versioned bucket", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "out-request-files"
			err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload"), []byte("contents"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			outRequest := out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          bucketName,
					RegionName:      regionName,
				},
				Params: out.Params{
					From: "file-to-upload",
					To:   directoryPrefix + "/",
				},
			}

			err = json.NewEncoder(stdin).Encode(&outRequest)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-to-upload"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("uploads the file to the correct bucket and outputs the version", func() {
			s3files, err := s3client.BucketFiles(bucketName, directoryPrefix)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "file-to-upload")))

			reader := bytes.NewBuffer(session.Buffer().Contents())

			var response out.OutResponse
			err = json.NewDecoder(reader).Decode(&response)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response).Should(Equal(out.OutResponse{
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "file-to-upload"),
				},
				Metadata: []s3resource.MetadataPair{
					{
						Name:  "filename",
						Value: "file-to-upload",
					},
					{
						Name:  "url",
						Value: "https://concourse-s3-resource-test-bucket-non-versioned.s3.amazonaws.com/out-request-files/file-to-upload",
					},
				},
			}))
		})
	})

	Context("with a versioned bucket", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "out-request-files-versioned"

			err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload"), []byte("contents"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			outRequest := out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
				},
				Params: out.Params{
					From: "file-to-upload",
					To:   directoryPrefix + "/",
				},
			}

			err = json.NewEncoder(stdin).Encode(&outRequest)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"))
			Ω(err).ShouldNot(HaveOccurred())

			for _, fileVersion := range fileVersions {
				err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"), fileVersion)
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		It("uploads the file to the correct bucket and outputs the version", func() {
			s3files, err := s3client.BucketFiles(versionedBucketName, directoryPrefix)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "file-to-upload")))

			reader := bytes.NewBuffer(session.Buffer().Contents())

			var response out.OutResponse
			err = json.NewDecoder(reader).Decode(&response)
			Ω(err).ShouldNot(HaveOccurred())

			versions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"))
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response).Should(Equal(out.OutResponse{
				Version: s3resource.Version{
					VersionID: versions[0],
				},
				Metadata: []s3resource.MetadataPair{
					{
						Name:  "filename",
						Value: "file-to-upload",
					},
					{
						Name:  "url",
						Value: "https://concourse-s3-resource-test-bucket-versioned.s3.amazonaws.com/out-request-files-versioned/file-to-upload?versionId=" + versions[0],
					},
				},
			}))
		})
	})
})
