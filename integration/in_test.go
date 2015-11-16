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
	"github.com/concourse/s3-resource/in"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("in", func() {
	var (
		command *exec.Cmd
		stdin   *bytes.Buffer
		session *gexec.Session
		destDir string

		expectedExitStatus int
	)

	BeforeEach(func() {
		var err error
		destDir, err = ioutil.TempDir("", "s3_in_integration_test")
		Ω(err).ShouldNot(HaveOccurred())

		stdin = &bytes.Buffer{}
		expectedExitStatus = 0

		command = exec.Command(inPath, destDir)
		command.Stdin = stdin
	})

	AfterEach(func() {
		err := os.RemoveAll(destDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		Eventually(session, 5*time.Second).Should(gexec.Exit(expectedExitStatus))
	})

	Context("with a versioned_file and a regex", func() {
		var inRequest in.InRequest

		BeforeEach(func() {
			inRequest = in.InRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
					Regexp:          "some-regex",
					VersionedFile:   "some-file",
				},
				Version: s3resource.Version{},
			}

			expectedExitStatus = 1

			err := json.NewEncoder(stdin).Encode(inRequest)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say("please specify either regexp or versioned_file"))
		})
	})

	Context("when the given version only has a path", func() {
		var inRequest in.InRequest
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "in-request-files"
			inRequest = in.InRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          bucketName,
					RegionName:      regionName,
					Regexp:          filepath.Join(directoryPrefix, "some-file-(.*)"),
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "some-file-2"),
				},
			}

			err := json.NewEncoder(stdin).Encode(inRequest)
			Ω(err).ShouldNot(HaveOccurred())

			tempFile, err := ioutil.TempFile("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-1"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "some-file-1"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-2"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "some-file-2"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-3"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "some-file-3"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "some-file-1"))
			Ω(err).ShouldNot(HaveOccurred())

			err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "some-file-2"))
			Ω(err).ShouldNot(HaveOccurred())

			err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "some-file-3"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("downloads the file", func() {
			reader := bytes.NewBuffer(session.Out.Contents())

			var response in.InResponse
			err := json.NewDecoder(reader).Decode(&response)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response).Should(Equal(in.InResponse{
				Version: s3resource.Version{
					Path: "in-request-files/some-file-2",
				},
				Metadata: []s3resource.MetadataPair{
					{
						Name:  "filename",
						Value: "some-file-2",
					},
					{
						Name:  "url",
						Value: buildEndpoint(bucketName, endpoint) + "/in-request-files/some-file-2",
					},
				},
			}))

			Ω(filepath.Join(destDir, "some-file-2")).Should(BeARegularFile())
			contents, err := ioutil.ReadFile(filepath.Join(destDir, "some-file-2"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("some-file-2")))

			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := ioutil.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte("2")))

			Ω(filepath.Join(destDir, "url")).Should(BeARegularFile())
			urlContents, err := ioutil.ReadFile(filepath.Join(destDir, "url"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(urlContents).Should(Equal([]byte(buildEndpoint(bucketName, endpoint) + "/in-request-files/some-file-2")))
		})
	})

	Context("when the given version has a versionID and path", func() {
		var inRequest in.InRequest
		var directoryPrefix string
		var expectedVersion string

		BeforeEach(func() {
			directoryPrefix = "in-request-files-versioned"
			inRequest = in.InRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
					VersionedFile:   filepath.Join(directoryPrefix, "some-file"),
				},
				Version: s3resource.Version{},
			}

			tempFile, err := ioutil.TempFile("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-1"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "some-file"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-2"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "some-file"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			err = ioutil.WriteFile(tempFile.Name(), []byte("some-file-3"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "some-file"), tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			versions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "some-file"))
			Ω(err).ShouldNot(HaveOccurred())
			expectedVersion = versions[1]
			inRequest.Version.VersionID = expectedVersion

			err = json.NewEncoder(stdin).Encode(inRequest)
			Ω(err).ShouldNot(HaveOccurred())

			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "some-file"))
			Ω(err).ShouldNot(HaveOccurred())

			for _, fileVersion := range fileVersions {
				err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "some-file"), fileVersion)
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		It("downloads the file", func() {
			reader := bytes.NewBuffer(session.Out.Contents())

			var response in.InResponse
			err := json.NewDecoder(reader).Decode(&response)

			Ω(response).Should(Equal(in.InResponse{
				Version: s3resource.Version{
					VersionID: expectedVersion,
				},
				Metadata: []s3resource.MetadataPair{
					{
						Name:  "filename",
						Value: "some-file",
					},
					{
						Name:  "url",
						Value: buildEndpoint(versionedBucketName, endpoint) + "/in-request-files-versioned/some-file?versionId=" + expectedVersion,
					},
				},
			}))

			Ω(filepath.Join(destDir, "some-file")).Should(BeARegularFile())
			contents, err := ioutil.ReadFile(filepath.Join(destDir, "some-file"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("some-file-2")))

			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := ioutil.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte(expectedVersion)))

			Ω(filepath.Join(destDir, "url")).Should(BeARegularFile())
			urlContents, err := ioutil.ReadFile(filepath.Join(destDir, "url"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(urlContents).Should(Equal([]byte(buildEndpoint(versionedBucketName, endpoint) + "/in-request-files-versioned/some-file?versionId=" + expectedVersion)))
		})
	})

})
