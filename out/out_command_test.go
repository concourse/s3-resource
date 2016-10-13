package out_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/fakes"
	"github.com/concourse/s3-resource/out"
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Out Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath   string
			sourceDir string
			request   out.OutRequest

			stderr   *gbytes.Buffer
			s3client *fakes.FakeS3Client
			command  *out.OutCommand
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = ioutil.TempDir("", "out_command")
			Ω(err).ShouldNot(HaveOccurred())

			sourceDir = filepath.Join(tmpPath, "source")
			err = os.MkdirAll(sourceDir, 0755)
			Ω(err).ShouldNot(HaveOccurred())

			request = out.OutRequest{
				Source: s3resource.Source{
					Bucket: "bucket-name",
				},
			}

			s3client = &fakes.FakeS3Client{}
			stderr = gbytes.NewBuffer()
			command = out.NewOutCommand(stderr, s3client)
		})

		AfterEach(func() {
			stderr.Close()
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

		Describe("finding files to upload with From param", func() {
			It("prints the deprecation warning", func() {
				request.Params.From = "foo.tgz"
				createFile("foo.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Expect(stderr.Contents()).To(ContainSubstring("WARNING:"))
				Expect(stderr.Contents()).To(ContainSubstring("Parameters 'from/to' are deprecated, use 'file' instead"))
			})

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

		Describe("finding files to upload with File param", func() {
			It("does not print the deprecation warning", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Expect(stderr.Contents()).NotTo(ContainSubstring("WARNING:"))
			})

			It("does not error if there is a single match", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("errors if there are no matches", func() {
				request.Params.File = "b/*.tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).Should(HaveOccurred())
			})

			It("errors if there are more than one match", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file1.tgz")
				createFile("a/file2.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).Should(HaveOccurred())
			})

			It("defaults the ACL to 'private'", func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")

				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))
			})
		})

		Context("when specifying an ACL for the uploaded file", func() {
			BeforeEach(func() {
				request.Params.File = "a/*.tgz"
				request.Params.Acl = "public-read"
				createFile("a/file.tgz")
			})

			It("applies the specfied acl", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "public-read"}))

			})
		})

		Context("when uploading the file with a To param", func() {
			BeforeEach(func() {
				request.Params.From = "a/(.*).tgz"
				request.Params.To = "a-folder/"
				createFile("a/file.tgz")
			})

			It("prints the deprecation warning", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Expect(stderr.Contents()).To(ContainSubstring("WARNING:"))
				Expect(stderr.Contents()).To(ContainSubstring("Parameters 'from/to' are deprecated, use 'file' instead"))
			})

			It("uploads the file", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("a-folder/file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))
			})
		})

		Context("when uploading the file with an empty To param", func() {
			BeforeEach(func() {
				request.Params.To = ""
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")
			})

			It("uploads the file to the root", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))
			})
		})

		Context("when uploading the file with a To param with templating", func() {
			BeforeEach(func() {
				request.Params.From = "a/file-(\\d*).tgz"
				request.Params.To = "folder-${1}/file.tgz"
				createFile("a/file-123.tgz")
			})

			It("uploads the file to the correct location", func() {
				response, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("folder-123/file.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, "a/file-123.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))

				Ω(response.Version.Path).Should(Equal("folder-123/file.tgz"))

				Ω(response.Metadata[0].Name).Should(Equal("filename"))
				Ω(response.Metadata[0].Value).Should(Equal("file.tgz"))
			})
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

				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal(remoteFileName))
				Ω(localPath).Should(Equal(filepath.Join(sourceDir, localFileName)))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))

				Ω(response.Version.VersionID).Should(Equal("123"))

				Ω(response.Metadata[0].Name).Should(Equal("filename"))
				Ω(response.Metadata[0].Value).Should(Equal(remoteFileName))
			})
		})

		Context("when using regexp", func() {
			It("uploads to the parent directory", func() {
				request.Params.File = "my/special-file.tgz"
				request.Source.Regexp = "a-folder/some-file-(.*).tgz"
				createFile("my/special-file.tgz")

				response, err := command.Run(sourceDir, request)
				Expect(err).ToNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))

				bucketName, remotePath, localPath, options := s3client.UploadFileArgsForCall(0)
				Expect(bucketName).To(Equal("bucket-name"))
				Expect(remotePath).To(Equal("a-folder/special-file.tgz"))
				Expect(localPath).To(Equal(filepath.Join(sourceDir, "my/special-file.tgz")))
				Ω(options).Should(Equal(s3resource.UploadFileOptions{Acl: "private"}))

				Expect(response.Metadata[0].Name).To(Equal("filename"))
				Expect(response.Metadata[0].Value).To(Equal("special-file.tgz"))
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

		Context("when specifying a content-type for the uploaded file", func() {
			BeforeEach(func() {
				request.Params.File = "a/*.tgz"
				request.Params.ContentType = "application/customtype"
				createFile("a/file.tgz")
			})

			It("applies the specfied content-type", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				_, _, _, options := s3client.UploadFileArgsForCall(0)

				Ω(options.ContentType).Should(Equal("application/customtype"))
			})
		})

		Context("content-type is not specified for the uploaded file", func() {
			BeforeEach(func() {
				request.Params.File = "a/*.tgz"
				createFile("a/file.tgz")
			})

			It("no content-type specified leaves an empty content-type", func() {
				_, err := command.Run(sourceDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.UploadFileCallCount()).Should(Equal(1))
				_, _, _, options := s3client.UploadFileArgsForCall(0)

				Ω(options.ContentType).Should(Equal(""))
			})
		})
	})
})
