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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
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
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "some-file-2"),
				},
			}

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			for i := 1; i <= 3; i++ {
				err = os.WriteFile(tempFile.Name(), []byte(fmt.Sprintf("some-file-%d", i)), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, fmt.Sprintf("some-file-%d", i)), tempFile.Name(), s3resource.NewUploadFileOptions())
				Ω(err).ShouldNot(HaveOccurred())
			}

			err = os.Remove(tempFile.Name())
			Ω(err).ShouldNot(HaveOccurred())
		})

		AfterEach(func() {
			for i := 1; i <= 3; i++ {
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
				},
				Version: s3resource.Version{},
			}

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			for i := 1; i <= 3; i++ {
				err = os.WriteFile(tempFile.Name(), []byte(fmt.Sprintf("some-file-%d", i)), 0755)
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

	Context("when cloudfront_url is set", func() {
		var directoryPrefix string

		BeforeEach(func() {
			if len(os.Getenv("S3_TESTING_CLOUDFRONT_URL")) == 0 {
				Skip("'S3_TESTING_CLOUDFRONT_URL' is not set, skipping.")
			}

			directoryPrefix = "in-request-cloudfront-files"
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					CloudfrontURL:   os.Getenv("S3_TESTING_CLOUDFRONT_URL"),
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          filepath.Join(directoryPrefix, "some-file-(.*)"),
				},
				Version: s3resource.Version{
					Path: filepath.Join(directoryPrefix, "some-file-2"),
				},
			}

			err := json.NewEncoder(stdin).Encode(inRequest)
			Ω(err).ShouldNot(HaveOccurred())

			tempFile, err := os.CreateTemp("", "file-to-upload")
			Ω(err).ShouldNot(HaveOccurred())
			tempFile.Close()

			for i := 1; i <= 3; i++ {
				err = os.WriteFile(tempFile.Name(), []byte(fmt.Sprintf("some-file-%d", i)), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				_, err = s3client.UploadFile(bucketName, filepath.Join(directoryPrefix, fmt.Sprintf("some-file-%d", i)), tempFile.Name(), s3resource.NewUploadFileOptions())
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		AfterEach(func() {
			for i := 1; i <= 3; i++ {
				err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, fmt.Sprintf("some-file-%d", i)))
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		It("downloads the file from CloudFront", func() {
			reader := bytes.NewBuffer(session.Out.Contents())

			var response in.Response
			err := json.NewDecoder(reader).Decode(&response)
			Ω(err).ShouldNot(HaveOccurred())

			Ω(response).Should(Equal(in.Response{
				Version: s3resource.Version{
					Path: "in-request-cloudfront-files/some-file-2",
				},
				Metadata: []s3resource.MetadataPair{
					{
						Name:  "filename",
						Value: "some-file-2",
					},
					{
						Name:  "url",
						Value: inRequest.Source.CloudfrontURL + "/in-request-cloudfront-files/some-file-2",
					},
				},
			}))

			Ω(filepath.Join(destDir, "some-file-2")).Should(BeARegularFile())
			contents, err := os.ReadFile(filepath.Join(destDir, "some-file-2"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(contents).Should(Equal([]byte("some-file-2")))

			Ω(filepath.Join(destDir, "url")).Should(BeARegularFile())
			urlContents, err := os.ReadFile(filepath.Join(destDir, "url"))
			Ω(err).ShouldNot(HaveOccurred())
			Ω(urlContents).Should(Equal([]byte(inRequest.Source.CloudfrontURL + "/in-request-cloudfront-files/some-file-2")))
		})
	})

	Context("when cloudfront_url is set but has too few dots", func() {
		BeforeEach(func() {
			inRequest = in.Request{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					SessionToken:    sessionToken,
					AwsRoleARN:      awsRoleARN,
					CloudfrontURL:   "https://no-dots-here",
					RegionName:      regionName,
					Endpoint:        endpoint,
					Regexp:          "unused",
				},
				Version: s3resource.Version{
					Path: "unused",
				},
			}

			expectedExitStatus = 1
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say(`'https://no-dots-here' doesn't have enough dots \('.'\), a typical format is 'https://d111111abcdef8.cloudfront.net'`))
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
})
