package out_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/gexec"
)

var outPath string

var _ = BeforeSuite(func() {
	var err error

	if _, err = os.Stat("/opt/resource/out"); err == nil {
		outPath = "/opt/resource/out"
	} else {
		outPath, err = gexec.Build("github.com/concourse/s3-resource/cmd/out")
		Ω(err).ShouldNot(HaveOccurred())
	}

})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()
})

func TestOut(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Out Suite")
}

func Fixture(filename string) string {
	path := filepath.Join("fixtures", filename)
	contents, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}

	return string(contents)
}
