package out_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/fakes"

	. "github.com/concourse/s3-resource/out"
)

var _ = Describe("Out Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath   string
			sourceDir string
			request   OutRequest

			s3client *fakes.FakeS3Client
			command  *OutCommand
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = ioutil.TempDir("", "out_command")
			Ω(err).ShouldNot(HaveOccurred())

			sourceDir = filepath.Join(tmpPath, "source")
			err = os.MkdirAll(sourceDir, 0755)
			Ω(err).ShouldNot(HaveOccurred())

			request = OutRequest{
				Source: s3resource.Source{
					Bucket: "bucket-name",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewOutCommand(s3client)
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpPath)
			Ω(err).ShouldNot(HaveOccurred())
		})

		createFile := func(path string) {
			fullPath := filepath.Join(sourceDir, path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			file, err := os.Create(fullPath)
			Ω(err).ShouldNot(HaveOccurred())
			file.Close()
		}

		Describe("finding files to upload", func() {
			It("does not error if there is a single match", func() {
				request.Params.From = "a/(.*).tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("errors if there are no matches", func() {
				request.Params.From = "b/(.*).tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).Should(HaveOccurred())
			})

			It("errors if there are more than one match", func() {
				request.Params.From = "a/(.*).tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).Should(HaveOccurred())
			})
		})

		Describe("uploading the file", func() {
			It("uploads the file", func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("a-folder/file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
			})

			It("can handle empty to to put it in the root", func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = ""
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
			})

			It("can handle templating in the output", func() {
				request.Params.From = "a/file-(\\d*).tgz"
				request.Params.To = "folder-${1}/file.tgz"
				createFile("a/file-123.tgz")

				response, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("folder-123/file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file-123.tgz")))

				Ω(response.Version.Path).Should(Equal("folder-123/file.tgz"))

				Ω(response.Metadata[0].Name).Should(Equal("filename"))
				Ω(response.Metadata[0].Value).Should(Equal("file.tgz"))
			})

			Context("when using versioned buckets", func() {
				BeforeEach(func() {
					s3client.UploadFileReturns("123", nil)
				})

				It("renames the local file to match the name of the versioned file", func() {
					localFileName := "not-the-same-name-as-versioned-file.tgz"
					remoteFileName := "versioned-file.tgz"

					request.Params.From = localFileName
					request.Source.VersionedFile = remoteFileName
					createFile(localFileName)

					response, err := command.Run(sourceDir, request)

					Ω(err).ShouldNot(HaveOccurred())

					Ω(s3client.UploadFileCallCount()).Should(Equal(1))
					bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

					Ω(bucketName).Should(Equal("bucket-name"))
					Ω(remotePath).Should(Equal(remoteFileName))
					Ω(localPath).Should(Equal(filepath.Join(sourceDir, localFileName)))

					Ω(response.Version.VersionID).Should(Equal("123"))

					Ω(response.Metadata[0].Name).Should(Equal("filename"))
					Ω(response.Metadata[0].Value).Should(Equal(remoteFileName))
				})
			})
		})

		Describe("output metadata", func() {
			BeforeEach(func() {
				s3client.URLStub = func(bucketName string, remotePath string, private bool, versionID string) string {
					return "http://example.com/" + filepath.Join(bucketName, remotePath)
				}
			})

			It("returns a response", func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")

				response, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.URLCallCount()).Should(Equal(1))
				bucketName, remotePath, private, versionID := s3client.URLArgsForCall(0)
				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("a-folder/file.tgz"))
				Ω(private).Should(Equal(false))
				Ω(versionID).Should(BeEmpty())

				Ω(response.Version.Path).Should(Equal("a-folder/file.tgz"))

				Ω(response.Metadata[0].Name).Should(Equal("filename"))
				Ω(response.Metadata[0].Value).Should(Equal("file.tgz"))

				Ω(response.Metadata[1].Name).Should(Equal("url"))
				Ω(response.Metadata[1].Value).Should(Equal("http://example.com/bucket-name/a-folder/file.tgz"))
			})

			It("doesn't include the URL if the output is private", func() {
				request.Source.Private = true
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")

				response, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response.Metadata).Should(HaveLen(1))
				Ω(response.Metadata[0].Name).ShouldNot(Equal("url"))
			})
		})
	})
})
