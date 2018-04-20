package check_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/fakes"

	. "github.com/concourse/s3-resource/check"
)

var _ = Describe("Check Command", func() {
	Describe("running the command", func() {
		var (
			tmpPath string
			request Request

			s3client *fakes.FakeS3Client
			command  *Command
		)

		BeforeEach(func() {
			var err error
			tmpPath, err = ioutil.TempDir("", "check_command")
			Ω(err).ShouldNot(HaveOccurred())

			request = Request{
				Source: s3resource.Source{
					Bucket: "bucket-name",
				},
			}

			s3client = &fakes.FakeS3Client{}
			command = NewCommand(s3client)

			s3client.BucketFilesReturns([]string{
				"files/abc-0.0.1.tgz",
				"files/abc-2.33.333.tgz",
				"files/abc-2.4.3.tgz",
				"files/abc-3.53.tgz",
			}, nil)
		})

		AfterEach(func() {
			err := os.RemoveAll(tmpPath)
			Ω(err).ShouldNot(HaveOccurred())
		})

		Context("when there is no previous version", func() {
			It("includes the latest version only", func() {
				request.Version.Path = ""
				request.Source.Regexp = "files/abc-(.*).tgz"

				response, err := command.Run(request)
				Ω(err).ShouldNot(HaveOccurred())

				Ω(response).Should(HaveLen(1))
				Ω(response).Should(ConsistOf(
					s3resource.Version{
						Path: "files/abc-3.53.tgz",
					},
				))
			})

			Context("when the initial version is set", func() {
				It("still returns the latest version", func() {
					request.Version.Path = ""
					request.Source.InitialPath = "files/abc-0.0.tgz"
					request.Source.Regexp = "files/abc-(.*).tgz"

					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(1))
					Ω(response).Should(ConsistOf(
						s3resource.Version{
							Path: "files/abc-3.53.tgz",
						},
					))
				})
			})

			Context("when the regexp does not match anything", func() {
				It("does not explode", func() {
					request.Source.Regexp = "no-files/missing-(.*).tgz"
					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(0))
				})

				Context("when the initial version is set", func() {
					It("returns the initial version", func() {
						request.Version.Path = ""
						request.Source.InitialPath = "no-files/missing-0.0.tgz"
						request.Source.Regexp = "no-files/missing-(.*).tgz"

						response, err := command.Run(request)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response).Should(HaveLen(1))
						Ω(response).Should(ConsistOf(
							s3resource.Version{
								Path: "no-files/missing-0.0.tgz",
							},
						))
					})
				})
			})

			Context("when the regex does not match the previous version", func() {
				It("returns the latest version that matches the regex", func() {
					request.Version.Path = "files/abc-0.0.1.tgz"
					request.Source.Regexp = `files/abc-(2\.33.*).tgz`
					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(1))
					Expect(response).To(ConsistOf(s3resource.Version{Path: "files/abc-2.33.333.tgz"}))
				})
			})
		})

		Context("when there is a previous version", func() {
			Context("when using regex that matches the provided version", func() {
				It("includes all versions from the previous one and the current one", func() {
					request.Version.Path = "files/abc-2.4.3.tgz"
					request.Source.Regexp = "files/abc-(.*).tgz"

					response, err := command.Run(request)
					Ω(err).ShouldNot(HaveOccurred())

					Ω(response).Should(HaveLen(3))
					Ω(response).Should(ConsistOf(
						s3resource.Version{
							Path: "files/abc-2.4.3.tgz",
						},
						s3resource.Version{
							Path: "files/abc-2.33.333.tgz",
						},
						s3resource.Version{
							Path: "files/abc-3.53.tgz",
						},
					))
				})
			})

			Context("when using versioned file", func() {
				Context("when there are existing versions", func() {
					BeforeEach(func() {
						s3client.BucketFileVersionsReturns([]string{
							"file-version-3",
							"file-version-2",
							"file-version-1",
						}, nil)
					})

					It("includes all versions from the previous one and the current one", func() {
						request.Version.VersionID = "file-version-2"
						request.Source.VersionedFile = "files/versioned-file"

						response, err := command.Run(request)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response).Should(HaveLen(2))
						Ω(response).Should(ConsistOf(
							s3resource.Version{
								VersionID: "file-version-2",
							},
							s3resource.Version{
								VersionID: "file-version-3",
							},
						))
					})
				})

				Context("when no version exists", func() {
					BeforeEach(func() {
						s3client.BucketFileVersionsReturns([]string{}, nil)
					})

					It("returns no versions", func() {
						request.Version.VersionID = ""
						request.Source.VersionedFile = "files/versioned-file"

						response, err := command.Run(request)
						Ω(err).ShouldNot(HaveOccurred())

						Ω(response).Should(HaveLen(0))
					})

					Context("when the initial version is set", func() {
						It("returns the initial version", func() {
							request.Version.VersionID = ""
							request.Source.VersionedFile = "files/versioned-file"
							request.Source.InitialVersion = "file-version-0"

							response, err := command.Run(request)
							Ω(err).ShouldNot(HaveOccurred())

							Ω(response).Should(HaveLen(1))
							Ω(response).Should(ConsistOf(
								s3resource.Version{
									VersionID: "file-version-0",
								},
							))
						})
					})
				})
			})
		})
	})
})
