package s3resource_test

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	s3resource "github.com/concourse/s3-resource"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AWSConfig", func() {
	Context("There are static credentials", func() {
		It("uses the static credentials", func() {
			accessKey := "access-key"
			secretKey := "secret-key"
			sessionToken := "session-token"
			cfg, err := s3resource.NewAwsConfig(accessKey, secretKey, sessionToken, "", "", false, false)
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
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Credentials).ToNot(BeNil())
			Expect(cfg.Credentials).To(Equal(aws.NewCredentialsCache(aws.AnonymousCredentials{})))
		})
	})

	Context("Set to use the Aws Default Credential Provider", func() {
		It("uses the Aws Default Credential Provider", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, true)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Credentials).ToNot(BeNil())
			Expect(cfg.Credentials).ToNot(Equal(aws.NewCredentialsCache(aws.AnonymousCredentials{})))
		})
	})

	Context("default values", func() {
		It("sets RetryMaxAttempts", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.RetryMaxAttempts).To(Equal(s3resource.MaxRetries))
		})

		It("sets region to us-east-1", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Region).To(Equal("us-east-1"))
		})

		It("uses aws buildable http client", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			_, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
			Expect(ok).To(BeTrue())
		})

		It("does not skip ssl verification", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			client, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
			Expect(ok).To(BeTrue())
			Expect(client.GetTransport().TLSClientConfig.InsecureSkipVerify).To(BeFalse())
		})
	})

	Context("Region is specified", func() {
		It("sets the region", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "ca-central-1", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Region).To(Equal("ca-central-1"))
		})
	})

	Context("SSL verification is skipped", func() {
		It("creates an http client that skips SSL verification", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", true, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())

			client, ok := cfg.HTTPClient.(*awshttp.BuildableClient)
			Expect(ok).To(BeTrue())
			Expect(client.GetTransport().TLSClientConfig.InsecureSkipVerify).To(BeTrue())
		})
	})

	Context("AWS_CA_BUNDLE is respected", func() {
		certificate := []byte("\n" +
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
			"-----END CERTIFICATE-----\n")

		defer BeforeEach(func() {
			bundleFile, err := os.CreateTemp("", "certbundle")
			Expect(err).NotTo(HaveOccurred())

			_, err = bundleFile.Write(certificate)
			Expect(err).NotTo(HaveOccurred())

			origCABundle := os.Getenv("AWS_CA_BUNDLE")
			err = os.Setenv("AWS_CA_BUNDLE", bundleFile.Name())
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				err := os.Setenv("AWS_CA_BUNDLE", origCABundle)
				Expect(err).NotTo(HaveOccurred())
				err = os.Remove(bundleFile.Name())
				Expect(err).NotTo(HaveOccurred())
			})
		})

		It("creates an http client that respects the AWS_CA_BUNDLE option", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false, false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())

			block, _ := pem.Decode(certificate)
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
