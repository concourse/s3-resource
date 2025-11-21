package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/in"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("in", func() {
	var (
		command            *exec.Cmd
		inRequest          in.Request
		stdin              *bytes.Buffer
		session            *gexec.Session
		destDir            string
		expectedExitStatus int
	)

	BeforeEach(func() {
		var err error
		destDir, err = os.MkdirTemp("", "s3_in_integration_test")
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

		err = json.NewEncoder(stdin).Encode(inRequest)
		Ω(err).ShouldNot(HaveOccurred())

		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		<-session.Exited
		Expect(session.ExitCode()).To(Equal(expectedExitStatus))
	})

	Context("with a versioned_file and a regex", func() {
		BeforeEach(func() {
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          "some-regex",
					VersionedFile:   "some-file",
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{},
			}

			expectedExitStatus = 1
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say("please specify either regexp or versioned_file"))
		})
	})

	Context("when the given version only has a path", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "in-request-files"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "some-file-(.*)"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "some-file-1"),
				},
			}

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			for i := range 3 {
				err = os.WriteFile(tempFile.Name(), fmt.Appendf([]byte{}, "some-file-%d", i), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, fmt.Sprintf("some-file-%d", i)), tempFile.Name(), s3resource.NewUploadFileOptions())
				Ω(err).ShouldNot(HaveOccurred())
			}

			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			for i := range 3 {
				err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, fmt.Sprintf("some-file-%d", i)))
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		It("downloads the file", func() {
			reader := bytes.NewBuffer(session.Out.Contents())

			var response in.Response
			err := json.NewDecoder(reader).Decode(&response)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response).Should(Equal(in.Response{
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
			contents, err := os.ReadFile(filepath.Join(destDir, "some-file-2"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("some-file-2")))

			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte("2")))

			Ω(filepath.Join(destDir, "url")).Should(BeARegularFile())
			urlContents, err := os.ReadFile(filepath.Join(destDir, "url"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(urlContents).Should(Equal([]byte(buildEndpoint(bucketName, endpoint) + "/in-request-files/some-file-2")))
		})

		Context("when the path matches the initial path", func() {
			BeforeEach(func() {
				inRequest.Source.InitialPath = filepath.Join(directoryPrefix, "some-file-0.0.0")
				inRequest.Source.InitialContentText = "initial content"
				inRequest.Version.Path = inRequest.Source.InitialPath
			})

			It("uses the initial content", func() {
				reader := bytes.NewBuffer(session.Out.Contents())

				var response in.Response
				err := json.NewDecoder(reader).Decode(&response)

				Ω(response).Should(Equal(in.Response{
					Version: s3resource.Version{
						Path: inRequest.Source.InitialPath,
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "some-file-0.0.0",
						},
					},
				}))

				Ω(filepath.Join(destDir, "some-file-0.0.0")).Should(BeARegularFile())
				contents, err := os.ReadFile(filepath.Join(destDir, "some-file-0.0.0"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(contents).Should(Equal([]byte(inRequest.Source.InitialContentText)))

				Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
				versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(versionContents).Should(Equal([]byte("0.0.0")))

				Ω(filepath.Join(destDir, "url")).ShouldNot(BeARegularFile())
			})
		})
	})

	Context("when the given version has a versionID and path", func() {
		var directoryPrefix string
		var expectedVersion string

		BeforeEach(func() {
			directoryPrefix = "in-request-files-versioned"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					VersionedFile:   filepath.Join(directoryPrefix, "some-file"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{},
			}

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			for i := range 3 {
				err = os.WriteFile(tempFile.Name(), fmt.Appendf([]byte{}, "some-file-%d", i), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = s3client.UploadFile(versionedBucketName, filepath.Join(directoryPrefix, "some-file"), tempFile.Name(), s3resource.NewUploadFileOptions())
				Ω(err).ShouldNot(HaveOccurred())
			}
			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			versions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "some-file"))
			Ω(err).ShouldNot(HaveOccurred())
			expectedVersion = versions[1]
			inRequest.Version.VersionID = expectedVersion
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

			var response in.Response
			err := json.NewDecoder(reader).Decode(&response)

			Ω(response).Should(Equal(in.Response{
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
			contents, err := os.ReadFile(filepath.Join(destDir, "some-file"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("some-file-2")))

			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte(expectedVersion)))

			Ω(filepath.Join(destDir, "url")).Should(BeARegularFile())
			urlContents, err := os.ReadFile(filepath.Join(destDir, "url"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(urlContents).Should(Equal([]byte(buildEndpoint(versionedBucketName, endpoint) + "/in-request-files-versioned/some-file?versionId=" + expectedVersion)))
		})

		Context("when the version ID matches the InitialVersion", func() {
			BeforeEach(func() {
				inRequest.Source.InitialVersion = "0.0.0"
				inRequest.Source.InitialContentText = "initial content"
				inRequest.Version.VersionID = inRequest.Source.InitialVersion
				expectedVersion = inRequest.Source.InitialVersion
			})

			It("uses the initial content", func() {
				reader := bytes.NewBuffer(session.Out.Contents())

				var response in.Response
				err := json.NewDecoder(reader).Decode(&response)

				Ω(response).Should(Equal(in.Response{
					Version: s3resource.Version{
						VersionID: inRequest.Source.InitialVersion,
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "some-file",
						},
					},
				}))

				Ω(filepath.Join(destDir, "some-file")).Should(BeARegularFile())
				contents, err := os.ReadFile(filepath.Join(destDir, "some-file"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(contents).Should(Equal([]byte(inRequest.Source.InitialContentText)))

				Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
				versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
				Ω(err).ShouldNot(HaveOccurred())
				Ω(versionContents).Should(Equal([]byte(expectedVersion)))

				Ω(filepath.Join(destDir, "url")).ShouldNot(BeARegularFile())
			})
		})
	})

	Context("when download_tags is true", func() {
		var (
			directoryPrefix string
			tags            map[string]string
		)

		BeforeEach(func() {
			directoryPrefix = "in-request-download-tags"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "some-file-(.*)"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "some-file-1"),
				},
				Params: in.Params{
					DownloadTags: true,
				},
			}

			err := json.NewEncoder(stdin).Encode(inRequest)
			Ω(err).ShouldNot(HaveOccurred())

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			err = os.WriteFile(tempFile.Name(), []byte("some-file-1"), 0755)
			Ω(err).ShouldNot(HaveOccurred())

			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "some-file-1"), tempFile.Name(), s3resource.NewUploadFileOptions())
			Ω(err).ShouldNot(HaveOccurred())

			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())

			tags = map[string]string{"tag1": "value1", "tag2": "value2"}
			err = s3client.SetTags(bucketName, filepath.Join(directoryPrefix, "some-file-1"), "", tags)
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "some-file-1"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("writes the tags to tags.json", func() {
			Ω(filepath.Join(destDir, "tags.json")).Should(BeARegularFile())
			actualTagsJSON, err := os.ReadFile(filepath.Join(destDir, "tags.json"))
			Ω(err).ShouldNot(HaveOccurred())

			expectedTagsJSON, err := json.Marshal(tags)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(actualTagsJSON).Should(MatchJSON(expectedTagsJSON))
		})
	})

	Context("when unpack is true with a .tar.bz2 file", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "in-request-unpack-bz2"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "archive-(.*)"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "archive-1.tar.bz2"),
				},
				Params: in.Params{
					Unpack: true,
				},
			}

			// Create temp directory for building the archive
			archiveTempDir, err := os.MkdirTemp("", "archive-build")
			Ω(err).ShouldNot(HaveOccurred())
			defer os.RemoveAll(archiveTempDir)

			// Create test files
			err = os.WriteFile(filepath.Join(archiveTempDir, "file1.txt"), []byte("content1"), 0644)
			Ω(err).ShouldNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(archiveTempDir, "file2.txt"), []byte("content2"), 0644)
			Ω(err).ShouldNot(HaveOccurred())
			err = os.Mkdir(filepath.Join(archiveTempDir, "subdir"), 0755)
			Ω(err).ShouldNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(archiveTempDir, "subdir", "file3.txt"), []byte("content3"), 0644)
			Ω(err).ShouldNot(HaveOccurred())

			// Create tar.bz2 archive
			archiveFile := filepath.Join(archiveTempDir, "archive-1.tar.bz2")
			tarCmd := exec.Command("tar", "-cjf", archiveFile, "-C", archiveTempDir, "file1.txt", "file2.txt", "subdir")
			err = tarCmd.Run()
			Ω(err).ShouldNot(HaveOccurred())

			// Upload to S3
			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "archive-1.tar.bz2"), archiveFile, s3resource.NewUploadFileOptions())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "archive-1.tar.bz2"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("extracts the archive contents", func() {
			// Verify the archive file was removed after extraction
			Ω(filepath.Join(destDir, "archive-1.tar.bz2")).ShouldNot(BeARegularFile())

			// Verify all files were extracted
			Ω(filepath.Join(destDir, "file1.txt")).Should(BeARegularFile())
			content1, err := os.ReadFile(filepath.Join(destDir, "file1.txt"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content1).Should(Equal([]byte("content1")))

			Ω(filepath.Join(destDir, "file2.txt")).Should(BeARegularFile())
			content2, err := os.ReadFile(filepath.Join(destDir, "file2.txt"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content2).Should(Equal([]byte("content2")))

			Ω(filepath.Join(destDir, "subdir", "file3.txt")).Should(BeARegularFile())
			content3, err := os.ReadFile(filepath.Join(destDir, "subdir", "file3.txt"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content3).Should(Equal([]byte("content3")))

			// Verify version file is created
			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte("1")))
		})
	})

	Context("when unpack is true with a .tar.gz file", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "in-request-unpack-gz"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "archive-(.*)"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "archive-1.tar.gz"),
				},
				Params: in.Params{
					Unpack: true,
				},
			}

			// Create temp directory for building the archive
			archiveTempDir, err := os.MkdirTemp("", "archive-build-gz")
			Ω(err).ShouldNot(HaveOccurred())
			defer os.RemoveAll(archiveTempDir)

			// Create test files
			err = os.WriteFile(filepath.Join(archiveTempDir, "gzfile1.txt"), []byte("gz-content1"), 0644)
			Ω(err).ShouldNot(HaveOccurred())
			err = os.WriteFile(filepath.Join(archiveTempDir, "gzfile2.txt"), []byte("gz-content2"), 0644)
			Ω(err).ShouldNot(HaveOccurred())

			// Create tar.gz archive
			archiveFile := filepath.Join(archiveTempDir, "archive-1.tar.gz")
			tarCmd := exec.Command("tar", "-czf", archiveFile, "-C", archiveTempDir, "gzfile1.txt", "gzfile2.txt")
			err = tarCmd.Run()
			Ω(err).ShouldNot(HaveOccurred())

			// Upload to S3
			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "archive-1.tar.gz"), archiveFile, s3resource.NewUploadFileOptions())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "archive-1.tar.gz"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("extracts the archive contents (regression test)", func() {
			// Verify the archive file was removed after extraction
			Ω(filepath.Join(destDir, "archive-1.tar.gz")).ShouldNot(BeARegularFile())

			// Verify files were extracted
			Ω(filepath.Join(destDir, "gzfile1.txt")).Should(BeARegularFile())
			content1, err := os.ReadFile(filepath.Join(destDir, "gzfile1.txt"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content1).Should(Equal([]byte("gz-content1")))

			Ω(filepath.Join(destDir, "gzfile2.txt")).Should(BeARegularFile())
			content2, err := os.ReadFile(filepath.Join(destDir, "gzfile2.txt"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content2).Should(Equal([]byte("gz-content2")))
		})
	})

	Context("when unpack is true with a plain .bz2 file", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "in-request-unpack-bz2-plain"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "file-(.*)"),
					UsePathStyle:    pathStyle,
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "file-1.bz2"),
				},
				Params: in.Params{
					Unpack: true,
				},
			}

			// Create temp directory for building the compressed file
			compressTempDir, err := os.MkdirTemp("", "compress-build")
			Ω(err).ShouldNot(HaveOccurred())
			defer os.RemoveAll(compressTempDir)

			// Create test file
			testFile := filepath.Join(compressTempDir, "file-1")
			err = os.WriteFile(testFile, []byte("plain bz2 content"), 0644)
			Ω(err).ShouldNot(HaveOccurred())

			// Compress with bzip2
			compressCmd := exec.Command("bzip2", "-k", testFile)
			err = compressCmd.Run()
			Ω(err).ShouldNot(HaveOccurred())

			// Upload to S3
			compressedFile := testFile + ".bz2"
			_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, "file-1.bz2"), compressedFile, s3resource.NewUploadFileOptions())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-1.bz2"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("decompresses the file", func() {
			// Verify the compressed file was removed after decompression
			Ω(filepath.Join(destDir, "file-1.bz2")).ShouldNot(BeARegularFile())

			// Verify decompressed file exists
			Ω(filepath.Join(destDir, "file-1")).Should(BeARegularFile())
			content, err := os.ReadFile(filepath.Join(destDir, "file-1"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(content).Should(Equal([]byte("plain bz2 content")))

			// Verify version file is created
			Ω(filepath.Join(destDir, "version")).Should(BeARegularFile())
			versionContents, err := os.ReadFile(filepath.Join(destDir, "version"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(versionContents).Should(Equal([]byte("1")))
		})
	})
})
