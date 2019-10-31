package imagefetcher_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"testing"
)

func TestImageFetcher(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ImageFetcher Suite")
}
