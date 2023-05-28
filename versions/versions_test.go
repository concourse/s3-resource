package versions_test

import (
	"errors"

	s3resource "github.com/concourse/s3-resource"
	"github.com/concourse/s3-resource/fakes"
	"github.com/concourse/s3-resource/versions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MatchFunc func(paths []string, pattern string) ([]string, error)

var ItMatchesPaths = func(matchFunc MatchFunc) {
	Describe("checking if paths in the bucket should be searched", func() {
		Context("when given an empty list of paths", func() {
			It("returns an empty list of matches", func() {
				result, err := matchFunc([]string{}, "regex")

				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})
		})

		Context("when given a single path", func() {
			It("returns it in a singleton list if it matches the regex", func() {
				paths := []string{"abc"}
				regex := "abc"

				result, err := matchFunc(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc"))
			})

			It("returns an empty list if it does not match the regexp", func() {
				paths := []string{"abc"}
				regex := "ad"

				result, err := matchFunc(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})

			It("accepts full regexes", func() {
				paths := []string{"abc"}
				regex := "a.*c"

				result, err := matchFunc(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc"))
			})

			It("errors when the regex is bad", func() {
				paths := []string{"abc"}
				regex := "a(c"

				_, err := matchFunc(paths, regex)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when given a multiple paths", func() {
			It("returns the matches", func() {
				paths := []string{"abc", "bcd"}
				regex := ".*bc.*"

				result, err := matchFunc(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc", "bcd"))
			})

			It("returns an empty list if none match the regexp", func() {
				paths := []string{"abc", "def"}
				regex := "ge.*h"

				result, err := matchFunc(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})
		})
	})
}

var _ = Describe("MatchUnanchored", func() {
	ItMatchesPaths(versions.MatchUnanchored)
})

var _ = Describe("ExtractSemver", func() {
	Context("when the path does not contain extractable information", func() {
		It("doesn't extract it", func() {
			result, ok := versions.ExtractSemver("abc.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeFalse())
			Ω(result).Should(BeZero())
		})
	})

	Context("when the path contains extractable information", func() {
		It("extracts it", func() {
			result, ok := versions.ExtractSemver("abc-105.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-105.tgz"))
			Ω(result.Version.String()).Should(Equal("105"))
			Ω(result.VersionNumber).Should(Equal("105"))
		})

		It("extracts semantic version numbers", func() {
			result, ok := versions.ExtractSemver("abc-1.0.5.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5.tgz"))
			Ω(result.Version.String()).Should(Equal("1.0.5"))
			Ω(result.VersionNumber).Should(Equal("1.0.5"))
		})

		It("extracts versions with more than 3 segments", func() {
			result, ok := versions.ExtractSemver("abc-1.0.6.1-rc7.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.VersionNumber).Should(Equal("1.0.6.1-rc7"))
			Ω(result.Version.String()).Should(Equal("1.0.6.1-rc7"))
		})

		It("takes the first match if there are many", func() {
			result, ok := versions.ExtractSemver("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.Version.String()).Should(Equal("1.0.5"))
			Ω(result.VersionNumber).Should(Equal("1.0.5"))
		})

		It("extracts a named group called 'version' above all others", func() {
			result, ok := versions.ExtractSemver("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(?P<version>.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.Version.String()).Should(Equal("2.3.4"))
			Ω(result.VersionNumber).Should(Equal("2.3.4"))
		})
	})
})

var _ = Describe("ExtractString", func() {
	Context("when the path does not contain extractable information", func() {
		It("doesn't extract it", func() {
			result, ok := versions.ExtractString("abc.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeFalse())
			Ω(result).Should(BeZero())
		})
	})

	Context("when the path contains extractable information", func() {
		It("extracts it", func() {
			result, ok := versions.ExtractString("abc-105.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-105.tgz"))
			Ω(result.VersionNumber).Should(Equal("105"))
		})
		It("extracts any pattern as a version", func() {
			result, ok := versions.ExtractString("abc-still-a-file.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())
			Ω(result.VersionNumber).Should(Equal("still-a-file"))
		})
		It("handles datetime-like files", func() {
			result, ok := versions.ExtractString("abc-2022-12-01.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())
			Ω(result.VersionNumber).Should(Equal("2022-12-01"))
		})
		It("extracts versions with more than 3 segments", func() {
			result, ok := versions.ExtractString("abc-1.0.6.1-rc7.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.VersionNumber).Should(Equal("1.0.6.1-rc7"))
		})

		It("takes the first match if there are many", func() {
			result, ok := versions.ExtractString("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.VersionNumber).Should(Equal("1.0.5"))
		})

		It("extracts a named group called 'version' above all others", func() {
			result, ok := versions.ExtractString("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(?P<version>.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.VersionNumber).Should(Equal("2.3.4"))
		})

		It("sorts files correctly", func() {
			result1, ok := versions.ExtractString("abc-2022-01-21.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())
			result2, ok := versions.ExtractString("abc-2022-12-01.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())
			Ω(result1.VersionNumber).Should(Equal("2022-01-21"))
			Ω(result2.VersionNumber).Should(Equal("2022-12-01"))
			Ω(result1.Compare(result2)).Should(Equal(-1))
		})
	})
})

var _ = Describe("GetMatchingPathsFromBucket", func() {
	var s3client *fakes.FakeS3Client

	BeforeEach(func() {
		s3client = &fakes.FakeS3Client{}
	})

	Context("When the regexp has no '/'", func() {
		Context("when the regexp has no special char", func() {
			It("uses only the empty string as prefix", func() {
				versions.GetMatchingPathsFromBucket(s3client, "bucket", "regexp")
				Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
				_, prefix, _ := s3client.ChunkedBucketListArgsForCall(0)
				Ω(prefix).Should(Equal(""))
			})
		})
		Context("when the regexp has a special char", func() {
			It("uses only the empty string as prefix", func() {
				versions.GetMatchingPathsFromBucket(s3client, "bucket", "reg.xp")
				Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
				_, prefix, _ := s3client.ChunkedBucketListArgsForCall(0)
				Ω(prefix).Should(Equal(""))
			})
		})
	})

	Context("When regexp special char appears close to the leaves", func() {
		It("starts directly with the longest prefix", func() {
			versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "regexp/will/appear/only/close/tw?o+/leaves",
			)
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
			_, prefix, _ := s3client.ChunkedBucketListArgsForCall(0)
			Ω(prefix).Should(Equal("regexp/will/appear/only/close/"))
		})

		It("follows only the matching prefixes", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				CommonPrefixes: []string{
					"regexp/will/appear/only/close/from/",
					"regexp/will/appear/only/close/to/",
					"regexp/will/appear/only/close/too/",
					"regexp/will/appear/only/close/two/",
				},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(1, s3resource.BucketListChunk{
				Paths: []string{
					"regexp/will/appear/only/close/to/the-end",
					"regexp/will/appear/only/close/to/leaves",
				},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(2, s3resource.BucketListChunk{
				CommonPrefixes: []string{"regexp/will/appear/only/close/too/late/"},
				Paths:          []string{"regexp/will/appear/only/close/too/soon"},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(3, s3resource.BucketListChunk{
				Paths: []string{
					"regexp/will/appear/only/close/two/three",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "regexp/will/appear/only/close/tw?o+/leaves",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(4))
			for idx, expectedPrefix := range []string{
				"regexp/will/appear/only/close/",
				"regexp/will/appear/only/close/to/",
				"regexp/will/appear/only/close/too/",
				"regexp/will/appear/only/close/two/",
			} {
				_, prefix, _ := s3client.ChunkedBucketListArgsForCall(idx)
				Ω(prefix).Should(Equal(expectedPrefix))
			}
			Ω(matchingPaths).Should(ConsistOf("regexp/will/appear/only/close/to/leaves"))
		})
	})

	Context("When there are too many leaves for a single request", func() {
		It("continues requesting more", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Truncated: true,
				Paths: []string{
					"prefix/leaf-0",
					"prefix/leaf-1",
				},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(1, s3resource.BucketListChunk{
				Truncated: false,
				Paths: []string{
					"prefix/leaf-2",
					"prefix/leaf-3",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "prefix/leaf-(.*)",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(matchingPaths).Should(ConsistOf(
				"prefix/leaf-0",
				"prefix/leaf-1",
				"prefix/leaf-2",
				"prefix/leaf-3",
			))
		})
	})

	Context("When there are too many prefixes for a single request", func() {
		It("continues requesting more", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Truncated: true,
				CommonPrefixes: []string{
					"prefix-0/",
					"prefix-1/",
				},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(1, s3resource.BucketListChunk{
				Truncated: false,
				CommonPrefixes: []string{
					"prefix-2/",
					"prefix-3/",
				},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(2, s3resource.BucketListChunk{
				Paths: []string{"prefix-0/leaf-0"},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(3, s3resource.BucketListChunk{
				Paths: []string{"prefix-1/leaf-1"},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(4, s3resource.BucketListChunk{
				Paths: []string{"prefix-2/leaf-2"},
			}, nil)
			s3client.ChunkedBucketListReturnsOnCall(5, s3resource.BucketListChunk{
				Paths: []string{"prefix-3/leaf-3"},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "prefix-\\d+/leaf-(.*)",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(matchingPaths).Should(ConsistOf(
				"prefix-0/leaf-0",
				"prefix-1/leaf-1",
				"prefix-2/leaf-2",
				"prefix-3/leaf-3",
			))
		})
	})

	Context("When regexp is not anchored explicitly and has no prefix", func() {
		It("will behave as if anchored at both ends", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Paths: []string{
					"substring",
					"also-substring",
					"subscribing",
					"substring.suffix",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "sub(.*)ing",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
			Ω(matchingPaths).Should(ConsistOf("substring", "subscribing"))
		})
	})

	Context("When regexp is not anchored explicitly and has prefix", func() {
		It("will behave as if anchored at both ends", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Paths: []string{
					"pre/ssing",
					"pre/singer",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "pre/(.*)ing",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
			_, prefix, _ := s3client.ChunkedBucketListArgsForCall(0)
			Ω(prefix).Should(Equal("pre/"))
			Ω(matchingPaths).Should(ConsistOf("pre/ssing"))
		})
	})

	Context("When regexp is anchored explicitly and has not prefix", func() {
		It("still works", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Paths: []string{
					"substring",
					"also-substring",
					"subscribing",
					"substring.suffix",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "^sub(.*)ing$",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
			Ω(matchingPaths).Should(ConsistOf("substring", "subscribing"))
		})
	})

	Context("When regexp is anchored explicitly and has prefix", func() {
		It("still works", func() {
			s3client.ChunkedBucketListReturnsOnCall(0, s3resource.BucketListChunk{
				Paths: []string{
					"pre/ssing",
					"pre/singer",
				},
			}, nil)

			matchingPaths, err := versions.GetMatchingPathsFromBucket(
				s3client, "bucket", "^pre/(.*)ing$",
			)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(s3client.ChunkedBucketListCallCount()).Should(Equal(1))
			_, prefix, _ := s3client.ChunkedBucketListArgsForCall(0)
			Ω(prefix).Should(Equal("pre/"))
			Ω(matchingPaths).Should(ConsistOf("pre/ssing"))
		})
	})

	Context("When S3 returns an error", func() {
		BeforeEach(func() {
			s3client.ChunkedBucketListReturns(
				s3resource.BucketListChunk{},
				errors.New("S3 failure"),
			)
		})
		It("fails", func() {
			_, err := versions.GetMatchingPathsFromBucket(s3client, "bucket", "dummy")
			Ω(err).Should(HaveOccurred())
		})
	})
})
