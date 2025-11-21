package s3resource_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	s3resource "github.com/concourse/s3-resource"
)

var _ = Describe("S3Resource", func() {
	Describe("AWSConfig", func() {
		Context("There are static credentials", func() {
			It("uses the static credentials", func() {
				accessKey := "access-key"
				secretKey := "secret-key"
				sessionToken := "session-token"
				cfg, err := s3resource.NewAwsConfig(accessKey, secretKey, sessionToken, "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())

				creds, err := cfg.Credentials.Retrieve(context.TODO())
				Expect(err).ToNot(HaveOccurred())
				Expect(creds).ToNot(BeNil())
				Expect(creds.AccessKeyID).To(Equal(accessKey))
				Expect(creds.SecretAccessKey).To(Equal(secretKey))
				Expect(creds.SessionToken).To(Equal(sessionToken))
			})
		})

		Context("There are no static credentials or role to assume", func() {
			It("uses the anonymous credentials", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Credentials).ToNot(BeNil())
				Expect(cfg.Credentials).To(Equal(aws.NewCredentialsCache(aws.AnonymousCredentials{})))
			})
		})

		Context("Set to use the Aws Default Credential Provider", func() {
			It("uses the Aws Default Credential Provider", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", true)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Credentials).ToNot(BeNil())
				Expect(cfg.Credentials).ToNot(Equal(aws.NewCredentialsCache(aws.AnonymousCredentials{})))
			})
		})

		Context("default values", func() {
			It("sets RetryMaxAttempts", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.RetryMaxAttempts).To(Equal(s3resource.MaxRetries))
			})

			It("sets region to us-east-1", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Region).To(Equal("us-east-1"))
			})

			It("uses aws buildable http client", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				_, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
				Expect(ok).To(BeTrue())
			})

			It("does not skip ssl verification", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				client, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
				Expect(ok).To(BeTrue())
				Expect(client.GetTransport().TLSClientConfig.InsecureSkipVerify).To(BeFalse())
			})
		})

		Context("Region is specified", func() {
			It("sets the region", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "ca-central-1", false, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())
				Expect(cfg.Region).To(Equal("ca-central-1"))
			})
		})

		Context("SSL verification is skipped", func() {
			It("creates an http client that skips SSL verification", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", true, "", false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())

				client, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
				Expect(ok).To(BeTrue())
				Expect(client.GetTransport().TLSClientConfig.InsecureSkipVerify).To(BeTrue())
			})
		})

		Context("ca_bundle option is respected", func() {
			certificate := "\n" +
				"-----BEGIN CERTIFICATE-----\n" +
				"MIIBrzCCAVmgAwIBAgIUbRo9f/LeC0cHVW708dsPek2H2qAwDQYJKoZIhvcNAQEL\n" +
				"BQAwLzELMAkGA1UEBhMCR0IxIDAeBgNVBAMMF1Rlc3RDZXJ0aWZpY2F0ZS5pbnZh\n" +
				"bGlkMB4XDTI1MDUzMDEwMTY1OFoXDTI2MDUzMDEwMTY1OFowLzELMAkGA1UEBhMC\n" +
				"R0IxIDAeBgNVBAMMF1Rlc3RDZXJ0aWZpY2F0ZS5pbnZhbGlkMFwwDQYJKoZIhvcN\n" +
				"AQEBBQADSwAwSAJBAOflTXmxKBPrQdergC/3iClfXdXl6tATr+i3u8CTeBjWngRE\n" +
				"QThS/arGhZVeQ++BBwfa2RRXcqQuvKdZaBrVxGUCAwEAAaNNMEswHQYDVR0OBBYE\n" +
				"FEZCf4s5sbaCGpaRnKvjIWPMBsj6MB8GA1UdIwQYMBaAFEZCf4s5sbaCGpaRnKvj\n" +
				"IWPMBsj6MAkGA1UdEwQCMAAwDQYJKoZIhvcNAQELBQADQQB0JwVRCCKFh4vxJToC\n" +
				"53Q2e9QuhKrRGtsLvaPOq0CLAlQBV+ufRl92CYblmpo6mINspqYzrOPRlcNk3kiu\n" +
				"57So\n" +
				"-----END CERTIFICATE-----\n"

			It("creates an http client that respects the ca_bundle option", func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, certificate, false)
				Expect(err).ToNot(HaveOccurred())
				Expect(cfg).ToNot(BeNil())

				block, _ := pem.Decode([]byte(certificate))
				Expect(block).ToNot(BeNil())
				Expect(block.Type).To(Equal("CERTIFICATE"))

				crt, err := x509.ParseCertificate(block.Bytes)
				Expect(err).ToNot(HaveOccurred())

				client, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
				Expect(ok).To(BeTrue())
				subjs := client.GetTransport().TLSClientConfig.RootCAs.Subjects()

				found := false
				for _, v := range subjs {
					if bytes.Equal(v, crt.RawSubject) {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("S3Client", func() {
		Describe("URL", func() {
			var (
				private   bool
				versionID string
				s3client  s3resource.S3Client
			)

			BeforeEach(func() {
				cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, "", false)
				Expect(err).ToNot(HaveOccurred())

				s3client, err = s3resource.NewS3Client(
					io.Discard,
					cfg,
					"fake-s3",
					false,
					true,
					true,
					"",
				)
			})

			Context("public", func() {
				BeforeEach(func() {
					private = false
				})

				It("Omits the versionId from the url if the object isn't versioned", func() {
					url, err := s3client.URL("bucketName", "remotePath", private, versionID)
					Expect(err).NotTo(HaveOccurred())
					Expect(url).To(Equal("https://fake-s3/bucketName/remotePath"))
				})

				Context("When the object has a version", func() {
					BeforeEach(func() {
						versionID = "some-version"
					})

					It("Correctly sets the versionId on the url", func() {
						url, err := s3client.URL("bucketName", "remotePath", private, versionID)
						Expect(err).NotTo(HaveOccurred())
						Expect(url).To(Equal("https://fake-s3/bucketName/remotePath?versionId=some-version"))
					})
				})
			})
		})
	})
})
