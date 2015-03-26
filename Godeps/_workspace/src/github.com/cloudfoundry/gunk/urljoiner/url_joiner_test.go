package urljoiner_test

import (
	. "github.com/cloudfoundry/gunk/urljoiner"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("UrlJoiner", func() {
	It("should join URLs", func() {
		Ω(Join("")).Should(Equal(""))
		Ω(Join("", "bar")).Should(Equal("/bar"))
		Ω(Join("http://foo.com")).Should(Equal("http://foo.com"))
		Ω(Join("http://foo.com/")).Should(Equal("http://foo.com/"))
		Ω(Join("http://foo.com", "bar")).Should(Equal("http://foo.com/bar"))
		Ω(Join("http://foo.com", "bar", "baz")).Should(Equal("http://foo.com/bar/baz"))
		Ω(Join("http://foo.com/", "bar", "/baz")).Should(Equal("http://foo.com/bar/baz"))
		Ω(Join("http://foo.com/", "/bar")).Should(Equal("http://foo.com/bar"))
		Ω(Join("http://foo.com", "")).Should(Equal("http://foo.com"))
		Ω(Join("http://foo.com/", "")).Should(Equal("http://foo.com/"))
	})
})
