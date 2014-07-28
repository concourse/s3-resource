package check_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/s3-resource/check"
)

// 1. get all paths
// 2. keep those which match regexp
// 3. extract value from paths
// 4. filter those which are less than or equal to the current version

var _ = Describe("Check", func() {
	Context("when given an empty list of paths", func() {
		It("returns an empty list of matches", func() {
			result, err := check.Match([]string{}, "regex")
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(BeEmpty())
		})
	})

	Context("when given a single path", func() {
		It("returns it in a singleton list if it matches the regex", func() {
			paths := []string{"abc"}
			regex := "ab"
			result, err := check.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(ConsistOf("abc"))
		})

		It("returns an empty list if it does not match the regexp", func() {
			paths := []string{"abc"}
			regex := "ad"
			result, err := check.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(BeEmpty())
		})

		It("accepts full regexes", func() {
			paths := []string{"abc"}
			regex := "a.*c"
			result, err := check.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(ConsistOf("abc"))
		})

		It("errors when the regex is bad", func() {
			paths := []string{"abc"}
			regex := "a(c"
			_, err := check.Match(paths, regex)
			Ω(err).Should(HaveOccurred())
		})
	})

	Context("when given a multiple paths", func() {
		It("returns the matches", func() {
			paths := []string{"abc", "bcd"}
			regex := ".*bc.*"
			result, err := check.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(ConsistOf("abc", "bcd"))
		})

		It("returns an empty list if if none match the regexp", func() {
			paths := []string{"abc", "def"}
			regex := "ge.*h"
			result, err := check.Match(paths, regex)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(result).Should(BeEmpty())
		})
	})
})
