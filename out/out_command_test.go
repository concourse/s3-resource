package out_test

import (
	"errors"
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
			Expect(err).ToNot(HaveOccurred())

			sourceDir = filepath.Join(tmpPath, "source")
			err = os.MkdirAll(sourceDir, 0755)
			Expect(err).ToNot(HaveOccurred())

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
			Expect(err).ToNot(HaveOccurred())
		})

		createFile := func(path string) {
			fullPath := filepath.Join(sourceDir, path)
			err := os.MkdirAll(filepath.Dir(fullPath), 0755)
			Expect(err).ToNot(HaveOccurred())

			file, err := os.Create(fullPath)
			Expect(err).ToNot(HaveOccurred())
			file.Close()
		}

		Describe("finding files to upload", func() {
			It("does not error if there is a single match", func() {
				request.Params.From = "a/(.*).tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())
			})

			It("errors if there are no matches", func() {
				request.Params.From = "b/(.*).tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).To(HaveOccurred())
			})

			It("errors if there are more than one match", func() {
				request.Params.From = "a/(.*).tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("finding files to upload with File param", func() {
			It("does not error if there is a single match", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())
			})

			It("errors if there are no matches", func() {
				request.Params.File = "b/*.tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).To(HaveOccurred())
			})

			It("errors if there are more than one match", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).To(HaveOccurred())
			})
		})

		Describe("uploading the file", func() {
			It("uploads the file", func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(s3client.UploadFileCallCount()).To(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Expect(bucketName).To(Equal("bucket-name"))
				Expect(remotePath).To(Equal("a-folder/file.tgz"))
				Expect(localPath).To(Equal(filepath.Join(sourceDir, "a/file.tgz")))
			})

			Describe("failure on uploading a file", func() {

				Context("aggressively retry", func() {
					BeforeEach(func() {
						request.Params.From = "a/(.*).tgz"
						request.Params.To = "a-folder/"
						createFile("a/file.tgz")
					})

					It("succeeds eventually", func() {
						counter := 0
						s3client.UploadFileStub = func(_, _, _ string) (string, error) {
							counter = counter + 1
							if counter < 10 {
								return "", errors.New("failed")
							} else {
								return "", nil
							}
						}

						_, err := command.Run(sourceDir, request)
						Expect(err).ToNot(HaveOccurred())
						Expect(s3client.UploadFileCallCount()).To(Equal(10))
					})

					It("fails in worst case", func() {
						s3client.UploadFileReturns("", errors.New("failed"))

						_, err := command.Run(sourceDir, request)
						Expect(err).To(HaveOccurred())
						Expect(s3client.UploadFileCallCount()).To(Equal(10))
					})
				})
			})

			It("can handle empty to to put it in the root", func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = ""
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())

				Expect(s3client.UploadFileCallCount()).To(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Expect(bucketName).To(Equal("bucket-name"))
				Expect(remotePath).To(Equal("file.tgz"))
				Expect(localPath).To(Equal(filepath.Join(sourceDir, "a/file.tgz")))
			})

			It("can handle templating in the output", func() {
				request.Params.From = "a/file-(\\d*).tgz"
				request.Params.To = "folder-${1}/file.tgz"
				createFile("a/file-123.tgz")

				response, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())

				Expect(s3client.UploadFileCallCount()).To(Equal(1))
				bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

				Expect(bucketName).To(Equal("bucket-name"))
				Expect(remotePath).To(Equal("folder-123/file.tgz"))
				Expect(localPath).To(Equal(filepath.Join(sourceDir, "a/file-123.tgz")))

				Expect(response.Version.Path).To(Equal("folder-123/file.tgz"))

				Expect(response.Metadata[0].Name).To(Equal("filename"))
				Expect(response.Metadata[0].Value).To(Equal("file.tgz"))
			})

			Context("when using versioned buckets", func() {
				BeforeEach(func() {
					s3client.UploadFileReturns("123", nil)
				})

				It("renames the local file to match the name of the versioned file", func() {
					localFileName := "not-the-same-name-as-versioned-file.tgz"
					remoteFileName := "versioned-file.tgz"

					request.Params.File = localFileName
					request.Source.VersionedFile = remoteFileName
					createFile(localFileName)

					response, err := command.Run(sourceDir, request)

					Expect(err).ToNot(HaveOccurred())

					Expect(s3client.UploadFileCallCount()).To(Equal(1))
					bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)

					Expect(bucketName).To(Equal("bucket-name"))
					Expect(remotePath).To(Equal(remoteFileName))
					Expect(localPath).To(Equal(filepath.Join(sourceDir, localFileName)))

					Expect(response.Version.VersionID).To(Equal("123"))

					Expect(response.Metadata[0].Name).To(Equal("filename"))
					Expect(response.Metadata[0].Value).To(Equal(remoteFileName))
				})
			})

			Context("when using regexp", func() {
				It("uploads to the parent directory", func() {
					request.Params.File = "my/special-file.tgz"
					request.Source.Regexp = "a-folder/some-file-(.*).tgz"
					createFile("my/special-file.tgz")

					response, err := command.Run(sourceDir, request)
					Expect(err).ToNot(HaveOccurred())

					Expect(s3client.UploadFileCallCount()).To(Equal(1))
					bucketName, remotePath, localPath := s3client.UploadFileArgsForCall(0)
					Expect(bucketName).To(Equal("bucket-name"))
					Expect(remotePath).To(Equal("a-folder/special-file.tgz"))
					Expect(localPath).To(Equal(filepath.Join(sourceDir, "my/special-file.tgz")))

					Expect(response.Metadata[0].Name).To(Equal("filename"))
					Expect(response.Metadata[0].Value).To(Equal("special-file.tgz"))
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
				Expect(err).ToNot(HaveOccurred())

				Expect(s3client.URLCallCount()).To(Equal(1))
				bucketName, remotePath, private, versionID := s3client.URLArgsForCall(0)
				Expect(bucketName).To(Equal("bucket-name"))
				Expect(remotePath).To(Equal("a-folder/file.tgz"))
				Expect(private).To(Equal(false))
				Expect(versionID).To(BeEmpty())

				Expect(response.Version.Path).To(Equal("a-folder/file.tgz"))

				Expect(response.Metadata[0].Name).To(Equal("filename"))
				Expect(response.Metadata[0].Value).To(Equal("file.tgz"))

				Expect(response.Metadata[1].Name).To(Equal("url"))
				Expect(response.Metadata[1].Value).To(Equal("http://example.com/bucket-name/a-folder/file.tgz"))
			})

			It("doesn't include the URL if the output is private", func() {
				request.Source.Private = true
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")

				response, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())

				Expect(response.Metadata).To(HaveLen(1))
				Expect(response.Metadata[0].Name).ToNot(Equal("url"))
			})
		})
	})
})
