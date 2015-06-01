package integration_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("S3client", func() {
	var (
		tempDir         string
		tempFile        *os.File
		runtime         string
		directoryPrefix string
	)

	BeforeEach(func() {
		var err error
		directoryPrefix = "s3client-tests"
		runtime = fmt.Sprintf("%d", time.Now().Unix())

		tempDir, err = ioutil.TempDir("", "s3-upload-dir")
		Ω(err).ShouldNot(HaveOccurred())

		tempFile, err = ioutil.TempFile(tempDir, "file-to-upload")
		Ω(err).ShouldNot(HaveOccurred())

		tempFile.Write([]byte("hello-" + runtime))
	})

	AfterEach(func() {

		err := os.RemoveAll(tempDir)
		Ω(err).ShouldNot(HaveOccurred())

		fileOneVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-1"))
		Ω(err).ShouldNot(HaveOccurred())

		for _, fileOneVersion := range fileOneVersions {
			err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-1"), fileOneVersion)
			Ω(err).ShouldNot(HaveOccurred())
		}

		fileTwoVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-2"))
		Ω(err).ShouldNot(HaveOccurred())

		for _, fileTwoVersion := range fileTwoVersions {
			err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-2"), fileTwoVersion)
			Ω(err).ShouldNot(HaveOccurred())
		}
	})

	It("Can interact with buckets", func() {
		_, err := s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-1"), tempFile.Name())
		Ω(err).ShouldNot(HaveOccurred())

		_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-2"), tempFile.Name())
		Ω(err).ShouldNot(HaveOccurred())

		_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-2"), tempFile.Name())
		Ω(err).ShouldNot(HaveOccurred())

		files, err := s3client.BucketFiles(versionedBucketName, directoryPrefix)
		Ω(err).ShouldNot(HaveOccurred())

		Ω(files).Should(ConsistOf([]string{filepath.Join(directoryPrefix, "file-to-upload-1"), filepath.Join(directoryPrefix, "file-to-upload-2")}))

		err = s3client.DownloadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload-1"), filepath.Join(tempDir, "downloaded-file"))
		Ω(err).ShouldNot(HaveOccurred())

		read, err := ioutil.ReadFile(filepath.Join(tempDir, "downloaded-file"))
		Ω(err).ShouldNot(HaveOccurred())
		Ω(read).Should(Equal([]byte("hello-" + runtime)))
	})
})
