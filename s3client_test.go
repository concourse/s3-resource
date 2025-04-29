package s3resource_test

import (
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
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
			cfg, err := s3resource.NewAwsConfig(accessKey, secretKey, sessionToken, "", "", false)
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
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Credentials).ToNot(BeNil())
			Expect(cfg.Credentials).To(Equal(aws.NewCredentialsCache(aws.AnonymousCredentials{})))
		})
	})

	Context("default values", func() {
		It("sets RetryMaxAttempts", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.RetryMaxAttempts).To(Equal(s3resource.MaxRetries))
		})

		It("sets region to us-east-1", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Region).To(Equal("us-east-1"))
		})

		It("uses the default http client", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.HTTPClient).To(Equal(http.DefaultClient))
		})
	})

	Context("Region is specified", func() {
		It("sets the region", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "ca-central-1", false)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())
			Expect(cfg.Region).To(Equal("ca-central-1"))
		})
	})

	Context("SSL verification is skipped", func() {
		It("creates a http client that skips SSL verification", func() {
			cfg, err := s3resource.NewAwsConfig("", "", "", "", "", true)
			Expect(err).ToNot(HaveOccurred())
			Expect(cfg).ToNot(BeNil())

			client, ok := cfg.HTTPClient.(*http.Client)
			Expect(ok).To(BeTrue())
			transport, ok := client.Transport.(*http.Transport)
			Expect(ok).To(BeTrue())
			Expect(transport.TLSClientConfig.InsecureSkipVerify).To(BeTrue())
		})
	})
})
