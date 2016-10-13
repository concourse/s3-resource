package integration_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/check"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("check", func() {
	var (
		command *exec.Cmd
		stdin   *bytes.Buffer
		session *gexec.Session

		expectedExitStatus int
	)

	BeforeEach(func() {
		stdin = &bytes.Buffer{}
		expectedExitStatus = 0

		command = exec.Command(checkPath)
		command.Stdin = stdin
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		<-session.Exited
		Expect(session.ExitCode()).To(Equal(expectedExitStatus))
	})

	Context("with a versioned_file and a regex", func() {
		var checkRequest check.CheckRequest

		BeforeEach(func() {
			checkRequest = check.CheckRequest{
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

			err := json.NewEncoder(stdin).Encode(checkRequest)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say("please specify either regexp or versioned_file"))
		})
	})

	Context("when we do not provide a previous version", func() {
		var directoryPrefix string
		var checkRequest check.CheckRequest

		Context("with a regex", func() {
			BeforeEach(func() {
				checkRequest = check.CheckRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          bucketName,
						RegionName:      regionName,
					},
					Version: s3resource.Version{},
				}
			})

			Context("with files in the bucket that do not match", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-not-match"

					checkRequest.Source.Regexp = filepath.Join(directoryPrefix, "file-does-match-(.*)")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-not-match-1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-not-match-1"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("returns an empty check response", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(BeEmpty())
				})
			})

			Context("with files in the bucket that match", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-match"
					checkRequest.Source.Regexp = filepath.Join(directoryPrefix, "file-does-match-(.*)")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-2"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-1"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-2"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("outputs the path of the latest versioned s3 object", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(Equal(check.CheckResponse{
						{
							Path: filepath.Join(directoryPrefix, "file-does-match-2"),
						},
					}))
				})
			})
		})

		Context("with a versioned_file", func() {
			BeforeEach(func() {
				checkRequest = check.CheckRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          versionedBucketName,
						RegionName:      regionName,
					},
					Version: s3resource.Version{},
				}
			})

			Context("and a bucket that does not have versioning enabled", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-match-version"
					expectedExitStatus = 1

					checkRequest.Source.Bucket = bucketName
					checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "versioned-file"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("returns an error", func() {
					Ω(session.Err).Should(gbytes.Say("bucket is not versioned"))
				})
			})

			Context("when the file does not exist", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-not-match"

					checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"))
					Ω(err).ShouldNot(HaveOccurred())

					for _, fileVersion := range fileVersions {
						err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"), fileVersion)
						Ω(err).ShouldNot(HaveOccurred())
					}
				})

				It("returns an empty check response", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(BeEmpty())
				})
			})

			Context("when the file exists", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-match"

					checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
					Ω(err).ShouldNot(HaveOccurred())

					for _, fileVersion := range fileVersions {
						err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), fileVersion)
						Ω(err).ShouldNot(HaveOccurred())
					}
				})

				It("returns the most recent version", func() {

					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(Equal(check.CheckResponse{
						{
							VersionID: fileVersions[0],
						},
					}))
				})
			})
		})
	})

	Context("when we provide a previous version", func() {
		var directoryPrefix string
		var checkRequest check.CheckRequest

		Context("with a regex", func() {
			BeforeEach(func() {
				checkRequest = check.CheckRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          bucketName,
						RegionName:      regionName,
					},
				}
			})

			Context("with files in the bucket that do not match", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-not-match-with-version"

					checkRequest.Source.Regexp = filepath.Join(directoryPrefix, "file-does-match-(.*)")
					checkRequest.Version.Path = "file-does-not-match-1"
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-not-match-1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-not-match-1"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("returns an empty check response", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(BeEmpty())
				})
			})

			Context("with files in the bucket that match", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-match-with-version"
					checkRequest.Source.Regexp = filepath.Join(directoryPrefix, "file-does-match-(.*)")
					checkRequest.Version.Path = filepath.Join(directoryPrefix, "file-does-match-2")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-2"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-3"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-1"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-2"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-does-match-3"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("outputs the path of the latest versioned s3 object", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(Equal(check.CheckResponse{
						{
							Path: filepath.Join(directoryPrefix, "file-does-match-2"),
						},
						{
							Path: filepath.Join(directoryPrefix, "file-does-match-3"),
						},
					}))
				})
			})

			Context("when the previous version does not match the regex", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-match-with-version"
					checkRequest.Source.Regexp = filepath.Join(directoryPrefix, `file-(1\.[2].*)`)
					checkRequest.Version.Path = filepath.Join(directoryPrefix, "file-1.1.0-rc.1")
					err := json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-1.2.0-rc.2"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-1.2.0-rc.1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-1.1.0-rc.1"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-1.1.0-rc.2"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-1.2.0-rc.2"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-1.2.0-rc.1"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-1.1.0-rc.1"))
					Ω(err).ShouldNot(HaveOccurred())

					err = s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-1.1.0-rc.2"))
					Ω(err).ShouldNot(HaveOccurred())
				})

				It("outputs the path of the latest versioned s3 object", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(Equal(check.CheckResponse{
						{
							Path: filepath.Join(directoryPrefix, "file-1.2.0-rc.2"),
						},
					}))
				})
			})

		})

		Context("with a versioned_file", func() {
			BeforeEach(func() {
				checkRequest = check.CheckRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          versionedBucketName,
						RegionName:      regionName,
					},
				}
			})

			Context("when the file does not exist", func() {
				BeforeEach(func() {
					directoryPrefix = "files-in-bucket-that-do-not-match-with-version"

					tempFile, err := ioutil.TempFile("", "file-to-upload")
					Ω(err).ShouldNot(HaveOccurred())
					tempFile.Close()

					_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"), tempFile.Name(), s3resource.NewUploadFileOptions())
					Ω(err).ShouldNot(HaveOccurred())

					err = os.Remove(tempFile.Name())
					Ω(err).ShouldNot(HaveOccurred())

					checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")

					fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"))
					Ω(err).ShouldNot(HaveOccurred())
					checkRequest.Version.VersionID = fileVersions[0]

					err = json.NewEncoder(stdin).Encode(checkRequest)
					Ω(err).ShouldNot(HaveOccurred())
				})

				AfterEach(func() {
					fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"))
					Ω(err).ShouldNot(HaveOccurred())

					for _, fileVersion := range fileVersions {
						err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-does-not-match"), fileVersion)
						Ω(err).ShouldNot(HaveOccurred())
					}
				})

				It("returns an empty check response", func() {
					reader := bytes.NewBuffer(session.Out.Contents())

					var response check.CheckResponse
					err := json.NewDecoder(reader).Decode(&response)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(BeEmpty())
				})
			})

			Context("when the file exists", func() {
				Context("when the version exists", func() {
					BeforeEach(func() {
						directoryPrefix = "files-in-bucket-that-do-match-with-version"

						tempFile, err := ioutil.TempFile("", "file-to-upload")
						Ω(err).ShouldNot(HaveOccurred())
						tempFile.Close()

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")

						fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
						Ω(err).ShouldNot(HaveOccurred())
						checkRequest.Version.VersionID = fileVersions[1]

						err = json.NewEncoder(stdin).Encode(checkRequest)
						Ω(err).ShouldNot(HaveOccurred())

						err = os.Remove(tempFile.Name())
						Ω(err).ShouldNot(HaveOccurred())
					})

					AfterEach(func() {
						fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
						Ω(err).ShouldNot(HaveOccurred())

						for _, fileVersion := range fileVersions {
							err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), fileVersion)
							Ω(err).ShouldNot(HaveOccurred())
						}
					})

					It("returns the most recent version", func() {
						reader := bytes.NewBuffer(session.Out.Contents())

						var response check.CheckResponse
						err := json.NewDecoder(reader).Decode(&response)
						Ω(err).ShouldNot(HaveOccurred())

						fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response).Should(Equal(check.CheckResponse{
							{
								VersionID: fileVersions[1],
							},
							{
								VersionID: fileVersions[0],
							},
						}))
					})
				})

				Context("When the version has been deleted", func() {
					var fileVersions []string

					BeforeEach(func() {
						directoryPrefix = "files-in-bucket-with-latest-version-deleted"

						tempFile, err := ioutil.TempFile("", "file-to-upload")
						Ω(err).ShouldNot(HaveOccurred())
						tempFile.Close()

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
						Ω(err).ShouldNot(HaveOccurred())

						checkRequest.Source.VersionedFile = filepath.Join(directoryPrefix, "versioned-file")

						fileVersions, err = s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
						Ω(err).ShouldNot(HaveOccurred())
						checkRequest.Version.VersionID = fileVersions[0]

						err = json.NewEncoder(stdin).Encode(checkRequest)
						Ω(err).ShouldNot(HaveOccurred())

						err = os.Remove(tempFile.Name())
						Ω(err).ShouldNot(HaveOccurred())

						err = s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), fileVersions[0])
						Ω(err).ShouldNot(HaveOccurred())
					})

					AfterEach(func() {
						fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"))
						Ω(err).ShouldNot(HaveOccurred())

						for _, fileVersion := range fileVersions {
							err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "versioned-file"), fileVersion)
							Ω(err).ShouldNot(HaveOccurred())
						}
					})

					It("returns the next most recent version", func() {
						reader := bytes.NewBuffer(session.Out.Contents())

						var response check.CheckResponse
						err := json.NewDecoder(reader).Decode(&response)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response).Should(Equal(check.CheckResponse{
							{
								VersionID: fileVersions[1],
							},
						}))
					})
				})
			})
		})
	})
})
