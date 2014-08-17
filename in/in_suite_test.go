package in_test

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/gomega/types"
)

func TestIn(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "In Suite")
}

func ExistOnFilesystem() types.GomegaMatcher {
	return &existOnFilesystemMatcher{}
}

type existOnFilesystemMatcher struct {
	expected interface{}
}

func (matcher *existOnFilesystemMatcher) Match(actual interface{}) (success bool, err error) {
	path, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("ExistOnFilesystem matcher expects a string")
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
	}

	return true, nil
}

func (matcher *existOnFilesystemMatcher) FailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nto exist on the filesystem", actual)
}

func (matcher *existOnFilesystemMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return fmt.Sprintf("Expected\n\t%#v\nnot to exist on the filesystem", actual)
}
