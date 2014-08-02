package versions_test

import (
	"github.com/concourse/s3-resource/versions"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// 1. get all paths
// 2. keep those which match regexp
// 3. extract value from paths
// 4. filter those which are less than or equal to the current version

var _ = Describe("Match", func() {
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
			regex := "ab"
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

		It("returns an empty list if if none match the regexp", func() {
			paths := []string{"abc", "def"}
			regex := "ge.*h"
			result, err := versions.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(BeEmpty())
		})
	})
})

var _ = Describe("Extract", func() {
	Context("when the path does not contain extractable information", func() {
		It("doesn't extract it", func() {
			result, ok := versions.Extract("abc.tgz")
			Ω(ok).Should(BeFalse())
			Ω(result).Should(BeZero())
		})
	})

	Context("when the path contains extractable information", func() {
		It("extracts it", func() {
			result, ok := versions.Extract("abc-105.tgz")
			Ω(ok).Should(BeTrue())

			Ω(result.Path).Should(Equal("abc-105.tgz"))
			Ω(result.Version).Should(Equal(105))
		})
	})
})
