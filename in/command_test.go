package in_test

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	s3resource "github.com/concourse/s3-resource"
	. "github.com/concourse/s3-resource/in"

	"github.com/concourse/s3-resource/fakes"
)

var _ = Describe("In Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath string
			destDir string
			request Request

			s3client *fakes.FakeS3Client
			command  *Command
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = os.MkdirTemp("", "in_command")
			Ω(err).ShouldNot(HaveOccurred())

			destDir = filepath.Join(tmpPath, "destination")
			request = Request{
				Source: s3resource.Source{
					Bucket: "bucket-name",
					Regexp: "files/a-file-(.*)",
				},
				Version: s3resource.Version{
					Path: "files/a-file-1.3",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewCommand(s3client)

			s3client.URLReturns("http://google.com", nil)
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

		Context("when there is no path in the requested version", func() {
			BeforeEach(func() {
				request.Version.Path = ""
			})

			It("returns an error", func() {
				_, err := command.Run(destDir, request)
				Expect(err).To(MatchError(ErrMissingPath))
			})
		})

		Context("when configured globally to skip download", func() {
			BeforeEach(func() {
				request.Source.SkipDownload = true
			})

			It("doesn't download the file", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s3client.DownloadFileCallCount()).Should(Equal(0))
			})
		})

		Context("when configured locally to skip download", func() {
			BeforeEach(func() {
				request.Params.SkipDownload = "true"
			})

			It("doesn't download the file", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s3client.DownloadFileCallCount()).Should(Equal(0))
			})
		})

		Context("when override locally to not skip download", func() {
			BeforeEach(func() {
				request.Source.SkipDownload = true
				request.Params.SkipDownload = "false"
			})

			It("doesn't download the file", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(s3client.DownloadFileCallCount()).Should(Equal(1))
			})
		})

		Context("when override using a wrong value for local skipdownload", func() {
			BeforeEach(func() {
				request.Params.SkipDownload = "foo"
			})

			It("doesn't download the file", func() {
				_, err := command.Run(destDir, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("skip_download defined but invalid value"))
			})
		})

		Context("when there is an existing version in the request", func() {
			BeforeEach(func() {
				request.Version.Path = "files/a-file-1.3"
			})

			It("downloads the existing version of the file", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3client.DownloadFileCallCount()).Should(Equal(1))
				bucketName, remotePath, versionID, localPath := s3client.DownloadFileArgsForCall(0)

				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-1.3"))
				Ω(versionID).Should(BeEmpty())
				Ω(localPath).Should(Equal(filepath.Join(destDir, "a-file-1.3")))
			})

			It("creates a 'url' file that contains the URL", func() {
				urlPath := filepath.Join(destDir, "url")
				Ω(urlPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(urlPath).Should(ExistOnFilesystem())
				contents, err := os.ReadFile(urlPath)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("http://google.com"))

				bucketName, remotePath, private, versionID := s3client.URLArgsForCall(0)
				Ω(bucketName).Should(Equal("bucket-name"))
				Ω(remotePath).Should(Equal("files/a-file-1.3"))
				Ω(private).Should(Equal(false))
				Ω(versionID).Should(BeEmpty())
			})

			It("creates a 's3_uri' file that contains the S3 URI", func() {
				uriPath := filepath.Join(destDir, "s3_uri")
				Ω(uriPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(uriPath).Should(ExistOnFilesystem())
				contents, err := os.ReadFile(uriPath)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("s3://" + request.Source.Bucket + "/files/a-file-1.3"))
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
					contents, err := os.ReadFile(urlPath)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("http://google.com"))

					Ω(s3client.URLCallCount()).Should(Equal(1))
					bucketName, remotePath, private, versionID := s3client.URLArgsForCall(0)
					Ω(bucketName).Should(Equal("bucket-name"))
					Ω(remotePath).Should(Equal("files/a-file-1.3"))
					Ω(private).Should(Equal(true))
					Ω(versionID).Should(BeEmpty())
				})
			})

			It("creates a 'version' file that contains the matched version", func() {
				versionFile := filepath.Join(destDir, "version")
				Ω(versionFile).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(versionFile).Should(ExistOnFilesystem())
				contents, err := os.ReadFile(versionFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal("1.3"))
			})

			Describe("the response", func() {
				It("has a version that is the remote file path", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Version.Path).Should(Equal("files/a-file-1.3"))
				})

				It("has metadata about the file", func() {
					response, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response.Metadata[0].Name).Should(Equal("filename"))
					Ω(response.Metadata[0].Value).Should(Equal("a-file-1.3"))

					Ω(response.Metadata[1].Name).Should(Equal("url"))
					Ω(response.Metadata[1].Value).Should(Equal("http://google.com"))
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

		Context("when the Regexp does not match the provided version", func() {
			BeforeEach(func() {
				request.Source.Regexp = "not-matching-anything"
			})

			It("returns an error", func() {
				_, err := command.Run(destDir, request)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("regex does not match provided version"))
				Expect(err.Error()).To(ContainSubstring("files/a-file-1.3"))
			})
		})

		Context("when params is configured to unpack the file", func() {
			BeforeEach(func() {
				request.Params.Unpack = true
			})

			Context("when the file is a tarball", func() {
				BeforeEach(func() {
					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						src := filepath.Join(tmpPath, "some-file")

						err := os.WriteFile(src, []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						err = createTarball([]string{src}, tmpPath, localPath)
						Expect(err).NotTo(HaveOccurred())

						_, err = os.Stat(localPath)
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("extracts the tarball", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					bs, err := os.ReadFile(filepath.Join(destDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())

					Expect(bs).To(Equal([]byte("some-contents")))
				})
			})

			Context("when the file is a zip", func() {
				BeforeEach(func() {
					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						inDir, err := os.MkdirTemp(tmpPath, "zip-dir")
						Expect(err).NotTo(HaveOccurred())

						err = os.WriteFile(path.Join(inDir, "some-file"), []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						err = zipit(path.Join(inDir, "/"), localPath, "")
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("unzips the zip", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					bs, err := os.ReadFile(filepath.Join(destDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())

					Expect(bs).To(Equal([]byte("some-contents")))
				})
			})

			Context("when the file is gzipped", func() {
				BeforeEach(func() {
					request.Version.Path = "files/a-file-1.3.gz"
					request.Source.Regexp = "files/a-file-(.*).gz"

					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						f, err := os.Create(localPath)
						Expect(err).NotTo(HaveOccurred())

						zw := gzip.NewWriter(f)

						_, err = zw.Write([]byte("some-contents"))
						Expect(err).NotTo(HaveOccurred())

						Expect(zw.Close()).NotTo(HaveOccurred())
						Expect(f.Close()).NotTo(HaveOccurred())

						return nil
					}
				})

				It("gunzips the gzip", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					bs, err := os.ReadFile(filepath.Join(destDir, "a-file-1.3"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(bs)).To(Equal("some-contents"))
				})
			})

			Context("when the file is a gzipped tarball", func() {
				BeforeEach(func() {
					request.Version.Path = "files/a-file-1.3.tgz"
					request.Source.Regexp = "files/a-file-(.*).tgz"

					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						err := os.MkdirAll(filepath.Join(tmpPath, "some-dir"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						someFile1 := filepath.Join(tmpPath, "some-dir", "some-file")

						err = os.WriteFile(someFile1, []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						someFile2 := filepath.Join(tmpPath, "some-file")

						err = os.WriteFile(someFile2, []byte("some-other-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						tarPath := filepath.Join(tmpPath, "some-tar")
						err = createTarball([]string{someFile1, someFile2}, tmpPath, tarPath)
						Expect(err).NotTo(HaveOccurred())

						_, err = os.Stat(tarPath)
						Expect(err).NotTo(HaveOccurred())

						tarf, err := os.Open(tarPath)
						Expect(err).NotTo(HaveOccurred())

						f, err := os.Create(localPath)
						Expect(err).NotTo(HaveOccurred())

						zw := gzip.NewWriter(f)

						_, err = io.Copy(zw, tarf)
						Expect(err).NotTo(HaveOccurred())

						Expect(zw.Close()).NotTo(HaveOccurred())
						Expect(f.Close()).NotTo(HaveOccurred())

						return nil
					}
				})

				It("extracts the gzipped tarball", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					Expect(filepath.Join(destDir, "some-dir", "some-file")).To(BeARegularFile())

					bs, err := os.ReadFile(filepath.Join(destDir, "some-dir", "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(bs).To(Equal([]byte("some-contents")))

					bs, err = os.ReadFile(filepath.Join(destDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(bs).To(Equal([]byte("some-other-contents")))
				})
			})

			Context("when the file is bzip2 compressed", func() {
				BeforeEach(func() {
					request.Version.Path = "files/a-file-1.3.bz2"
					request.Source.Regexp = "files/a-file-(.*).bz2"

					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						// Create uncompressed file
						uncompressedPath := filepath.Join(tmpPath, "uncompressed-file")
						err := os.WriteFile(uncompressedPath, []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						// Compress with bzip2 command
						cmd := exec.Command("bzip2", "-c", uncompressedPath)
						compressed, err := cmd.Output()
						Expect(err).NotTo(HaveOccurred())

						// Write compressed data to localPath
						err = os.WriteFile(localPath, compressed, os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("decompresses the bzip2 file", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					bs, err := os.ReadFile(filepath.Join(destDir, "a-file-1.3"))
					Expect(err).NotTo(HaveOccurred())

					Expect(string(bs)).To(Equal("some-contents"))
				})
			})

			Context("when the file is a bzip2 compressed tarball", func() {
				BeforeEach(func() {
					request.Version.Path = "files/a-file-1.3.tar.bz2"
					request.Source.Regexp = "files/a-file-(.*).tar.bz2"

					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						// Create directory structure
						err := os.MkdirAll(filepath.Join(tmpPath, "some-dir"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						someFile1 := filepath.Join(tmpPath, "some-dir", "some-file")
						err = os.WriteFile(someFile1, []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						someFile2 := filepath.Join(tmpPath, "some-file")
						err = os.WriteFile(someFile2, []byte("some-other-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						// Create tarball
						tarPath := filepath.Join(tmpPath, "some-tar")
						err = createTarball([]string{someFile1, someFile2}, tmpPath, tarPath)
						Expect(err).NotTo(HaveOccurred())

						_, err = os.Stat(tarPath)
						Expect(err).NotTo(HaveOccurred())

						// Read the tar file
						tarData, err := os.ReadFile(tarPath)
						Expect(err).NotTo(HaveOccurred())

						// Compress with bzip2 command
						cmd := exec.Command("bzip2", "-c")
						cmd.Stdin = strings.NewReader(string(tarData))
						compressed, err := cmd.Output()
						Expect(err).NotTo(HaveOccurred())

						// Write compressed tar to localPath
						err = os.WriteFile(localPath, compressed, os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("extracts the bzip2 compressed tarball", func() {
					_, err := command.Run(destDir, request)
					Expect(err).NotTo(HaveOccurred())

					Expect(filepath.Join(destDir, "some-dir", "some-file")).To(BeARegularFile())

					bs, err := os.ReadFile(filepath.Join(destDir, "some-dir", "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(bs).To(Equal([]byte("some-contents")))

					bs, err = os.ReadFile(filepath.Join(destDir, "some-file"))
					Expect(err).NotTo(HaveOccurred())
					Expect(bs).To(Equal([]byte("some-other-contents")))
				})
			})

			Context("when the file is not an archive", func() {
				BeforeEach(func() {
					s3client.DownloadFileStub = func(bucketName string, remotePath string, versionID string, localPath string) error {
						err := os.WriteFile(localPath, []byte("some-contents"), os.ModePerm)
						Expect(err).NotTo(HaveOccurred())

						return nil
					}
				})

				It("returns an error", func() {
					_, err := command.Run(destDir, request)
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("when the requested path is the initial path", func() {
			var initialFilename string

			BeforeEach(func() {
				initialFilename = "a-file-0.0"
				request.Source.InitialPath = "files/a-file-0.0"
				request.Version.Path = request.Source.InitialPath
				request.Source.InitialContentText = "the hard questions are hard 🙈"
			})

			It("it creates a file containing the initial text content", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				contentFile := filepath.Join(destDir, initialFilename)
				Ω(contentFile).Should(BeARegularFile())
				contents, err := os.ReadFile(contentFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal(request.Source.InitialContentText))
			})

			Context("when the initial content is binary", func() {
				BeforeEach(func() {
					request.Source.InitialContentText = ""
					request.Source.InitialContentBinary = "dGhlIGhhcmQgcXVlc3Rpb25zIGFyZSBoYXJkIPCfmYg="
				})
				It("it creates a file containing the initial binary content", func() {
					_, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					contentFile := filepath.Join(destDir, initialFilename)
					Ω(contentFile).Should(BeARegularFile())
					contents, err := os.ReadFile(contentFile)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("the hard questions are hard 🙈"))
				})

				Context("when base64 decoding fails", func() {
					BeforeEach(func() {
						request.Source.InitialContentBinary = "not base64 data 🙈"
					})
					It("should return with an error", func() {
						_, err := command.Run(destDir, request)
						Ω(err).Should(HaveOccurred())
					})
				})
			})

			It("should not write the URL file", func() {
				urlPath := filepath.Join(destDir, "url")
				Ω(urlPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(urlPath).ShouldNot(ExistOnFilesystem())
			})

			It("should not write the s3_uri file", func() {
				uriPath := filepath.Join(destDir, "s3_uri")
				Ω(uriPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(uriPath).ShouldNot(ExistOnFilesystem())
			})

			It("should not include a URL in the metadata", func() {
				response, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				for _, metadatum := range response.Metadata {
					Ω(metadatum.Name).ShouldNot(Equal("url"))
				}
			})

			It("should not attempt to unpack the initial content", func() {
				request.Params.Unpack = true
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				contentFile := filepath.Join(destDir, initialFilename)
				Ω(contentFile).Should(BeARegularFile())
				contents, err := os.ReadFile(contentFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal(request.Source.InitialContentText))
			})
		})

		Context("when the requested version is the initial version", func() {
			var filename = "testfile"

			BeforeEach(func() {
				request.Source.Regexp = ""
				request.Source.VersionedFile = "file/testfile"
				request.Source.InitialVersion = "0.0.0"
				request.Version.VersionID = request.Source.InitialVersion
				request.Source.InitialContentText = "the hard questions are hard 🙈"
			})

			It("it creates a file containing the initial text content", func() {
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				contentFile := filepath.Join(destDir, filename)
				Ω(contentFile).Should(BeARegularFile())
				contents, err := os.ReadFile(contentFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal(request.Source.InitialContentText))
			})

			Context("when the initial content is binary", func() {
				BeforeEach(func() {
					request.Source.InitialContentText = ""
					request.Source.InitialContentBinary = "dGhlIGhhcmQgcXVlc3Rpb25zIGFyZSBoYXJkIPCfmYg="
				})
				It("it creates a file containing the initial binary content", func() {
					_, err := command.Run(destDir, request)
					Ω(err).ShouldNot(HaveOccurred())

					contentFile := filepath.Join(destDir, filename)
					Ω(contentFile).Should(BeARegularFile())
					contents, err := os.ReadFile(contentFile)
					Ω(err).ShouldNot(HaveOccurred())
					Ω(string(contents)).Should(Equal("the hard questions are hard 🙈"))
				})

				Context("when base64 decoding fails", func() {
					BeforeEach(func() {
						request.Source.InitialContentBinary = "not base64 data 🙈"
					})
					It("should return with an error", func() {
						_, err := command.Run(destDir, request)
						Ω(err).Should(HaveOccurred())
					})
				})
			})

			It("should not write the URL file", func() {
				urlPath := filepath.Join(destDir, "url")
				Ω(urlPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(urlPath).ShouldNot(ExistOnFilesystem())
			})

			It("should not write the s3_uri file", func() {
				uriPath := filepath.Join(destDir, "s3_uri")
				Ω(uriPath).ShouldNot(ExistOnFilesystem())

				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(uriPath).ShouldNot(ExistOnFilesystem())
			})

			It("should not include a URL in the metadata", func() {
				response, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				for _, metadatum := range response.Metadata {
					Ω(metadatum.Name).ShouldNot(Equal("url"))
				}
			})

			It("should not attempt to unpack the initial content", func() {
				request.Params.Unpack = true
				_, err := command.Run(destDir, request)
				Ω(err).ShouldNot(HaveOccurred())

				contentFile := filepath.Join(destDir, filename)
				Ω(contentFile).Should(BeARegularFile())
				contents, err := os.ReadFile(contentFile)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(string(contents)).Should(Equal(request.Source.InitialContentText))
			})
		})
	})
})

func addFileToTar(tw *tar.Writer, tarPath, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	err = tw.WriteHeader(&tar.Header{
		Name:    tarPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	})
	if err != nil {
		return err
	}

	_, err = io.Copy(tw, file)
	if err != nil {
		return err
	}

	return file.Close()
}

func createTarball(paths []string, basePath string, destination string) error {
	file, err := os.Create(destination)
	if err != nil {
		log.Fatalln(err)
	}

	tw := tar.NewWriter(file)

	for _, path := range paths {
		tarPath, err := filepath.Rel(basePath, path)
		if err != nil {
			return err
		}
		err = addFileToTar(tw, tarPath, path)
		if err != nil {
			return err
		}
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	return file.Close()
}

// Thanks to Svett Ralchev
// http://blog.ralch.com/tutorial/golang-working-with-zip/
func zipit(source, target, prefix string) error {
	zipfile, err := os.Create(target)
	if err != nil {
		return err
	}

	archive := zip.NewWriter(zipfile)

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if path == source {
			return nil
		}

		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		header.Name = strings.TrimPrefix(path, source+string(os.PathSeparator))

		if info.IsDir() {
			header.Name += string(os.PathSeparator)
		} else {
			header.Method = zip.Deflate
		}

		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}

		if _, err = io.Copy(writer, file); err != nil {
			return err
		}

		return file.Close()
	})

	if err = archive.Close(); err != nil {
		return err
	}

	return zipfile.Close()
}
