package integration_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/out"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"github.com/nu7hatch/gouuid"
)

var _ = Describe("out", func() {
	var (
		command   *exec.Cmd
		stdin     *bytes.Buffer
		session   *gexec.Session
		sourceDir string

		expectedExitStatus int
	)

	BeforeEach(func() {
		var err error
		sourceDir, err = ioutil.TempDir("", "s3_out_integration_test")
		Ω(err).ShouldNot(HaveOccurred())

		stdin = &bytes.Buffer{}
		expectedExitStatus = 0

		command = exec.Command(outPath, sourceDir)
		command.Stdin = stdin
	})

	AfterEach(func() {
		err := os.RemoveAll(sourceDir)
		Ω(err).ShouldNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		var err error
		session, err = gexec.Start(command, GinkgoWriter, GinkgoWriter)
		Ω(err).ShouldNot(HaveOccurred())

		<-session.Exited
		Expect(session.ExitCode()).To(Equal(expectedExitStatus))
	})

	Context("with a versioned_file and a regex", func() {
		var outRequest out.OutRequest

		BeforeEach(func() {
			outRequest = out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          versionedBucketName,
					RegionName:      regionName,
					Regexp:          "some-regex",
					VersionedFile:   "some-file",
					Endpoint:        endpoint,
				},
			}

			expectedExitStatus = 1

			err := json.NewEncoder(stdin).Encode(outRequest)
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say("please specify either regexp or versioned_file"))
		})
	})

	Context("with a content-type", func() {
		BeforeEach(func() {
			ioutil.WriteFile(filepath.Join(sourceDir, "content-typed-file"), []byte("text only"), 0755)

			outRequest := out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
				},
				Params: out.Params{
					From:        filepath.Join(sourceDir, "content-typed-file"),
					To:          "",
					ContentType: "application/customtype",
					Acl:         "public-read",
				},
			}

			err := json.NewEncoder(stdin).Encode(&outRequest)
			Ω(err).ShouldNot(HaveOccurred())

			expectedExitStatus = 0
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, "content-typed-file")
			Ω(err).ShouldNot(HaveOccurred())
		})

		It("creates a file with the specified content-type", func() {
			response, err := s3Service.HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String("content-typed-file"),
			})
			Ω(err).ShouldNot(HaveOccurred())

			Expect(response.ContentType).To(Equal(aws.String("application/customtype")))
		})
	})

	Context("without a content-type", func() {
		BeforeEach(func() {
			ioutil.WriteFile(filepath.Join(sourceDir, "uncontent-typed-file"), []byte("text only"), 0755)

			outRequest := out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
				},
				Params: out.Params{
					From: filepath.Join(sourceDir, "uncontent-typed-file"),
					To:   "",
					Acl:  "public-read",
				},
			}

			err := json.NewEncoder(stdin).Encode(&outRequest)
			Ω(err).ShouldNot(HaveOccurred())

			expectedExitStatus = 0
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, "uncontent-typed-file")
			Ω(err).ShouldNot(HaveOccurred())
		})

		// http://docs.aws.amazon.com/AWSImportExport/latest/DG/FileExtensiontoMimeTypes.html
		It("creates a file with the default S3 content-type for a unknown filename extension", func() {
			response, err := s3Service.HeadObject(&s3.HeadObjectInput{
				Bucket: aws.String(bucketName),
				Key:    aws.String("uncontent-typed-file"),
			})
			Ω(err).ShouldNot(HaveOccurred())

			Expect(response.ContentType).To(Equal(aws.String("binary/octet-stream")))
		})
	})

	Context("with a file glob and from", func() {
		BeforeEach(func() {
			outRequest := out.OutRequest{
				Source: s3resource.Source{
					AccessKeyID:     accessKeyID,
					SecretAccessKey: secretAccessKey,
					Bucket:          bucketName,
					RegionName:      regionName,
					Endpoint:        endpoint,
				},
				Params: out.Params{
					File: "glob-*",
					From: "file-to-upload-local",
					To:   "/",
				},
			}

			err := json.NewEncoder(stdin).Encode(&outRequest)
			Ω(err).ShouldNot(HaveOccurred())

			expectedExitStatus = 1
		})

		It("returns an error", func() {
			Ω(session.Err).Should(gbytes.Say("contains both file and from"))
		})
	})

	Context("with a non-versioned bucket", func() {
		var directoryPrefix string

		BeforeEach(func() {
			guid, err := uuid.NewV4()
			Expect(err).ToNot(HaveOccurred())
			directoryPrefix = "out-request-files-" + guid.String()
		})

		AfterEach(func() {
			err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "file-to-upload"))
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("with a file glob and public read ACL specified", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(sourceDir, "glob-file-to-upload"), []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				outRequest := out.OutRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          bucketName,
						RegionName:      regionName,
						Endpoint:        endpoint,
					},
					Params: out.Params{
						File: "glob-*",
						To:   directoryPrefix + "/",
						Acl:  "public-read",
					},
				}

				err = json.NewEncoder(stdin).Encode(&outRequest)
				Ω(err).ShouldNot(HaveOccurred())
			})

			AfterEach(func() {
				err := s3client.DeleteFile(bucketName, filepath.Join(directoryPrefix, "glob-file-to-upload"))
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("uploads the file to the correct bucket and outputs the version", func() {
				s3files, err := s3client.BucketFiles(bucketName, directoryPrefix)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "glob-file-to-upload")))

				reader := bytes.NewBuffer(session.Buffer().Contents())

				var response out.OutResponse
				err = json.NewDecoder(reader).Decode(&response)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(Equal(out.OutResponse{
					Version: s3resource.Version{
						Path: filepath.Join(directoryPrefix, "glob-file-to-upload"),
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "glob-file-to-upload",
						},
						{
							Name:  "url",
							Value: buildEndpoint(bucketName, endpoint) + "/" + directoryPrefix + "/glob-file-to-upload",
						},
					},
				}))
			})

			It("allows everyone to have read access to the object", func() {
				anonURI := "http://acs.amazonaws.com/groups/global/AllUsers"
				permision := s3.PermissionRead
				grantee := s3.Grantee{URI: &anonURI}
				expectedGrant := s3.Grant{
					Grantee:    &grantee,
					Permission: &permision,
				}

				params := &s3.GetObjectAclInput{
					Bucket: aws.String(bucketName),
					Key:    aws.String(filepath.Join(directoryPrefix, "glob-file-to-upload")),
				}

				resp, err := s3Service.GetObjectAcl(params)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(resp.Grants).Should(ContainElement(&expectedGrant))
			})
		})

		Context("with regexp", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload"), []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				outRequest := out.OutRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          bucketName,
						RegionName:      regionName,
						Endpoint:        endpoint,
						Regexp:          filepath.Join(directoryPrefix, "some-file-pattern"),
					},
					Params: out.Params{
						File: "file-to-upload",
					},
				}

				err = json.NewEncoder(stdin).Encode(&outRequest)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("uploads the file to the correct bucket and outputs the version", func() {
				s3files, err := s3client.BucketFiles(bucketName, directoryPrefix)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "file-to-upload")))

				reader := bytes.NewBuffer(session.Out.Contents())

				var response out.OutResponse
				err = json.NewDecoder(reader).Decode(&response)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(Equal(out.OutResponse{
					Version: s3resource.Version{
						Path: filepath.Join(directoryPrefix, "file-to-upload"),
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "file-to-upload",
						},
						{
							Name:  "url",
							Value: buildEndpoint(bucketName, endpoint) + "/" + directoryPrefix + "/file-to-upload",
						},
					},
				}))
			})
		})

		Context("with versioned_file", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload-local"), []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				outRequest := out.OutRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          bucketName,
						RegionName:      regionName,
						VersionedFile:   filepath.Join(directoryPrefix, "file-to-upload"),
						Endpoint:        endpoint,
					},
					Params: out.Params{
						From: "file-to-upload-local",
						To:   "something-wrong/",
					},
				}

				expectedExitStatus = 1

				err = json.NewEncoder(stdin).Encode(&outRequest)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("reports that it failed to create a versioned object", func() {
				Ω(session.Err).Should(gbytes.Say("object versioning not enabled"))
			})
		})
	})

	Context("with a versioned bucket", func() {
		var directoryPrefix string

		BeforeEach(func() {
			directoryPrefix = "out-request-files-versioned"
		})

		AfterEach(func() {
			fileVersions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"))
			Ω(err).ShouldNot(HaveOccurred())

			for _, fileVersion := range fileVersions {
				err := s3client.DeleteVersionedFile(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"), fileVersion)
				Ω(err).ShouldNot(HaveOccurred())
			}
		})

		Context("with versioned_file", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload-local"), []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				outRequest := out.OutRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          versionedBucketName,
						RegionName:      regionName,
						VersionedFile:   filepath.Join(directoryPrefix, "file-to-upload"),
						Endpoint:        endpoint,
					},
					Params: out.Params{
						From: "file-to-upload-local",
						To:   "something-wrong/",
					},
				}

				err = json.NewEncoder(stdin).Encode(&outRequest)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("uploads the file to the correct bucket and outputs the version", func() {
				s3files, err := s3client.BucketFiles(versionedBucketName, directoryPrefix)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "file-to-upload")))

				reader := bytes.NewBuffer(session.Out.Contents())

				var response out.OutResponse
				err = json.NewDecoder(reader).Decode(&response)
				Ω(err).ShouldNot(HaveOccurred())

				versions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(Equal(out.OutResponse{
					Version: s3resource.Version{
						VersionID: versions[0],
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "file-to-upload",
						},
						{
							Name:  "url",
							Value: buildEndpoint(versionedBucketName, endpoint) + "/" + directoryPrefix + "/file-to-upload?versionId=" + versions[0],
						},
					},
				}))
			})
		})

		Context("with regexp", func() {
			BeforeEach(func() {
				err := ioutil.WriteFile(filepath.Join(sourceDir, "file-to-upload"), []byte("contents"), 0755)
				Ω(err).ShouldNot(HaveOccurred())

				outRequest := out.OutRequest{
					Source: s3resource.Source{
						AccessKeyID:     accessKeyID,
						SecretAccessKey: secretAccessKey,
						Bucket:          versionedBucketName,
						RegionName:      regionName,
						Endpoint:        endpoint,
					},
					Params: out.Params{
						From: "file-to-upload",
						To:   directoryPrefix + "/",
					},
				}

				err = json.NewEncoder(stdin).Encode(&outRequest)
				Ω(err).ShouldNot(HaveOccurred())
			})

			It("uploads the file to the correct bucket and outputs the version", func() {
				s3files, err := s3client.BucketFiles(versionedBucketName, directoryPrefix)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(s3files).Should(ConsistOf(filepath.Join(directoryPrefix, "file-to-upload")))

				reader := bytes.NewBuffer(session.Out.Contents())

				var response out.OutResponse
				err = json.NewDecoder(reader).Decode(&response)
				Ω(err).ShouldNot(HaveOccurred())

				versions, err := s3client.BucketFileVersions(versionedBucketName, filepath.Join(directoryPrefix, "file-to-upload"))
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(Equal(out.OutResponse{
					Version: s3resource.Version{
						Path: filepath.Join(directoryPrefix, "file-to-upload"),
					},
					Metadata: []s3resource.MetadataPair{
						{
							Name:  "filename",
							Value: "file-to-upload",
						},
						{
							Name:  "url",
							Value: buildEndpoint(versionedBucketName, endpoint) + "/" + directoryPrefix + "/file-to-upload?versionId=" + versions[0],
						},
					},
				}))
			})
		})
	})
})
