package versions_test

import (
	"github.com/concourse/s3-resource/versions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type MatchFunc func(paths []string, pattern string) ([]string, error)

var ItMatchesPaths = func(matchFunc MatchFunc) {
	Describe("checking if paths in the bucket should be searched", func() {
		Context("when given an empty list of paths", func() {
			It("returns an empty list of matches", func() {
				result, err := versions.Match([]string{}, "regex")

				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})
		})

		Context("when given a single path", func() {
			It("returns it in a singleton list if it matches the regex", func() {
				paths := []string{"abc"}
				regex := "abc"

				result, err := versions.Match(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc"))
			})

			It("returns an empty list if it does not match the regexp", func() {
				paths := []string{"abc"}
				regex := "ad"

				result, err := versions.Match(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})

			It("accepts full regexes", func() {
				paths := []string{"abc"}
				regex := "a.*c"

				result, err := versions.Match(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc"))
			})

			It("errors when the regex is bad", func() {
				paths := []string{"abc"}
				regex := "a(c"

				_, err := versions.Match(paths, regex)
				Ω(err).Should(HaveOccurred())
			})
		})

		Context("when given a multiple paths", func() {
			It("returns the matches", func() {
				paths := []string{"abc", "bcd"}
				regex := ".*bc.*"

				result, err := versions.Match(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(ConsistOf("abc", "bcd"))
			})

			It("returns an empty list if none match the regexp", func() {
				paths := []string{"abc", "def"}
				regex := "ge.*h"

				result, err := versions.Match(paths, regex)
				Ω(err).ShouldNot(HaveOccurred())
				Ω(result).Should(BeEmpty())
			})
		})
	})
}

var _ = Describe("MatchUnanchored", func() {
	ItMatchesPaths(versions.MatchUnanchored)
})

var _ = Describe("Extract", func() {
	Context("when the path does not contain extractable information", func() {
		It("doesn't extract it", func() {
			result, ok := versions.Extract("abc.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeFalse())
			Ω(result).Should(BeZero())
		})
	})

	Context("when the path contains extractable information", func() {
		It("extracts it", func() {
			result, ok := versions.Extract("abc-105.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-105.tgz"))
			Ω(result.Version.String()).Should(Equal("105"))
			Ω(result.VersionNumber).Should(Equal("105"))
		})

		It("extracts semantic version numbers", func() {
			result, ok := versions.Extract("abc-1.0.5.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5.tgz"))
			Ω(result.Version.String()).Should(Equal("1.0.5"))
			Ω(result.VersionNumber).Should(Equal("1.0.5"))
		})

		It("extracts versions with more than 3 segments", func() {
			result, ok := versions.Extract("abc-1.0.6.1-rc7.tgz", "abc-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.VersionNumber).Should(Equal("1.0.6.1-rc7"))
			Ω(result.Version.String()).Should(Equal("1.0.6.1-rc7"))
		})

		It("takes the first match if there are many", func() {
			result, ok := versions.Extract("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.Version.String()).Should(Equal("1.0.5"))
			Ω(result.VersionNumber).Should(Equal("1.0.5"))
		})

		It("extracts a named group called 'version' above all others", func() {
			result, ok := versions.Extract("abc-1.0.5-def-2.3.4.tgz", "abc-(.*)-def-(?P<version>.*).tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-1.0.5-def-2.3.4.tgz"))
			Ω(result.Version.String()).Should(Equal("2.3.4"))
			Ω(result.VersionNumber).Should(Equal("2.3.4"))
		})
	})
})
