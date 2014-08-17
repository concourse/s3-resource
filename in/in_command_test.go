package in_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource"
	. "github.com/concourse/s3-resource/in"

	"github.com/concourse/s3-resource/fakes"
)

var _ = Describe("In Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath string
			destDir string
			request InRequest

			s3client *fakes.FakeS3Client
			command  *InCommand
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = ioutil.TempDir("", "in_command")
			Ω(err).ShouldNot(HaveOccurred())

			destDir = filepath.Join(tmpPath, "destination")
			request = InRequest{
				Source: s3resource.Source{
					Bucket: "bucket-name",
				},
				Version: s3resource.Version{
					Path: "files/a-file-1.3.tgz",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewInCommand(s3client)
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpPath)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("creates the destination directory", func() {
			Ω(destDir).ShouldNot(ExistOnFilesystem())

			_, err := command.Run(destDir, request)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(destDir).Should(ExistOnFilesystem())
		})

		Context("when there is no existing version in the request", func() {
			BeforeEach(func() {
				request.Version.Path = ""
				request.Source.Regexp = "files/abc-(.*).tgz"

				s3client.BucketFilesReturns([]string{
					"files/abc-0.0.1.tgz",
					"files/abc-3.53.tgz",
					"files/abc-2.33.333.tgz",
					"files/abc-2.4.3.tgz",
				}, nil)
			})

			It("scans the bucket for the latest file to download", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.DownloadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.DownloadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/abc-3.53.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(destDir, "abc-3.53.tgz")))
			})

			It("returns an error when the regexp has no groups", func() {
				request.Source.Regexp = "files/abc-.*.tgz"

				_, err := command.Run(destDir, request)
				Ω(err).Should(HaveOccurred())
			})

			Describe("the response", func() {
				It("has a version that is the remote file path", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Version.Path).Should(Equal("files/abc-3.53.tgz"))
				})

				It("has metadata about the file", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Metadata[0].Name).Should(Equal("filename"))
					Ω(response.Metadata[0].Value).Should(Equal("abc-3.53.tgz"))
				})
			})
		})

		Context("when there is an existing version in the request", func() {
			BeforeEach(func() {
				request.Version.Path = "files/a-file-1.3.tgz"
			})

			It("downloads the existing version of the file", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				bucketName, remotePath, localPath := s3client.DownloadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-1.3.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(destDir, "a-file-1.3.tgz")))
			})

			Describe("the response", func() {
				It("has a version that is the remote file path", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Version.Path).Should(Equal("files/a-file-1.3.tgz"))
				})

				It("has metadata about the file", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Metadata[0].Name).Should(Equal("filename"))
					Ω(response.Metadata[0].Value).Should(Equal("a-file-1.3.tgz"))
				})
			})
		})
	})
})
