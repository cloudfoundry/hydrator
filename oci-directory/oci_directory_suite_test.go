package directory_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestOciDirectory(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "OciDirectory Suite")
}
