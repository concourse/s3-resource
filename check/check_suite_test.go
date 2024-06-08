package check_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var checkPath string

var _ = BeforeSuite(func() {
	var err error

	if _, err = os.Stat("/opt/resource/check"); err == nil {
		checkPath = "/opt/resource/check"
	} else {
		checkPath, err = gexec.Build("github.com/concourse/s3-resource/cmd/check")
		Ω(err).ShouldNot(HaveOccurred())
	}

})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestCheck(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Check Suite")
}

func Fixture(filename string) string {
	path := filepath.Join("fixtures", filename)
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(contents)
}
