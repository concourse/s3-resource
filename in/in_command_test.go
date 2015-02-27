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
					Regexp: "files/a-file-(.*).tgz",
				},
				Version: s3resource.Version{
					Path: "files/a-file-1.3.tgz",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewInCommand(s3client)

			s3client.URLReturns("google.com")
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

				s3client.BucketFilesReturns([]string{
					"files/a-file-0.0.1.tgz",
					"files/a-file-3.53.tgz",
					"files/a-file-2.33.333.tgz",
					"files/a-file-2.4.3.tgz",
				}, nil)
			})

			It("scans the bucket for the latest file to download", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.DownloadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.DownloadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-3.53.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(destDir, "a-file-3.53.tgz")))
			})

			It("creates a 'url' file that contains the URL", func() {
				urlPath := filepath.Join(destDir, "url")
				Ω(urlPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(urlPath).Should(ExistOnFilesystem())
				contents, err := ioutil.ReadFile(urlPath)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("google.com"))

				bucketName, remotePath, private := s3client.URLArgsForCall(0)
				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-3.53.tgz"))
				Ω(private).Should(Equal(false))
			})

			Context("when configured with private URLs", func() {
				BeforeEach(func() {
					request.Source.Private = true
				})

				It("creates a 'url' file that contains the private URL if told to do that", func() {
					urlPath := filepath.Join(destDir, "url")
					Ω(urlPath).ShouldNot(ExistOnFilesystem())

					_, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(urlPath).Should(ExistOnFilesystem())
					contents, err := ioutil.ReadFile(urlPath)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("google.com"))

					Ω(s3client.URLCallCount()).Should(Equal(1))
					bucketName, remotePath, private := s3client.URLArgsForCall(0)
					Ω(bucketName).Should(Equal("bucket-name"))
					Ω(remotePath).Should(Equal("files/a-file-3.53.tgz"))
					Ω(private).Should(Equal(true))
				})
			})

			It("creates a 'version' file that contains the latest version", func() {
				versionFile := filepath.Join(destDir, "version")
				Ω(versionFile).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(versionFile).Should(ExistOnFilesystem())
				contents, err := ioutil.ReadFile(versionFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("3.53"))
			})

			Context("when the regexp has no groups", func() {
				BeforeEach(func() {
					request.Source.Regexp = "files/a-file-.*.tgz"
				})

				It("returns an error when the regexp has no groups", func() {
					_, err := command.Run(destDir, request)
					Ω(err).Should(HaveOccurred())
				})
			})

			Describe("the response", func() {
				It("has a version that is the remote file path", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Version.Path).Should(Equal("files/a-file-3.53.tgz"))
				})

				It("has metadata about the file", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Metadata[0].Name).Should(Equal("filename"))
					Ω(response.Metadata[0].Value).Should(Equal("a-file-3.53.tgz"))

					Ω(response.Metadata[1].Name).Should(Equal("url"))
					Ω(response.Metadata[1].Value).Should(Equal("google.com"))
				})

				Context("when the output is private", func() {
					BeforeEach(func() {
						request.Source.Private = true
					})

					It("doesn't include the URL in the metadata", func() {
						response, err := command.Run(destDir, request)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.Metadata).Should(HaveLen(1))
						Ω(response.Metadata[0].Name).ShouldNot(Equal("url"))
					})
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

				Ω(s3client.DownloadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, localPath := s3client.DownloadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-1.3.tgz"))
				Ω(localPath).Should(Equal(filepath.Join(destDir, "a-file-1.3.tgz")))
			})

			It("creates a 'url' file that contains the URL", func() {
				urlPath := filepath.Join(destDir, "url")
				Ω(urlPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(urlPath).Should(ExistOnFilesystem())
				contents, err := ioutil.ReadFile(urlPath)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("google.com"))

				bucketName, remotePath, private := s3client.URLArgsForCall(0)
				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-1.3.tgz"))
				Ω(private).Should(Equal(false))
			})

			Context("when configured with private URLs", func() {
				BeforeEach(func() {
					request.Source.Private = true
				})

				It("creates a 'url' file that contains the private URL if told to do that", func() {
					urlPath := filepath.Join(destDir, "url")
					Ω(urlPath).ShouldNot(ExistOnFilesystem())

					_, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(urlPath).Should(ExistOnFilesystem())
					contents, err := ioutil.ReadFile(urlPath)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("google.com"))

					Ω(s3client.URLCallCount()).Should(Equal(1))
					bucketName, remotePath, private := s3client.URLArgsForCall(0)
					Ω(bucketName).Should(Equal("bucket-name"))
					Ω(remotePath).Should(Equal("files/a-file-1.3.tgz"))
					Ω(private).Should(Equal(true))
				})
			})

			It("creates a 'version' file that contains the matched version", func() {
				versionFile := filepath.Join(destDir, "version")
				Ω(versionFile).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(versionFile).Should(ExistOnFilesystem())
				contents, err := ioutil.ReadFile(versionFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("1.3"))
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

					Ω(response.Metadata[1].Name).Should(Equal("url"))
					Ω(response.Metadata[1].Value).Should(Equal("google.com"))
				})

				Context("when the output is private", func() {
					BeforeEach(func() {
						request.Source.Private = true
					})

					It("doesn't include the URL in the metadata", func() {
						response, err := command.Run(destDir, request)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response.Metadata).Should(HaveLen(1))
						Ω(response.Metadata[0].Name).ShouldNot(Equal("url"))
					})
				})
			})
		})
	})
})
