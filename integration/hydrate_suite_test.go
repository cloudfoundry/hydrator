package hydrate_test

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/hydrator/imagefetcher"
	testhelpers "code.cloudfoundry.org/hydrator/testhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestHydrate(t *testing.T) {
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(time.Second * 300)
	RunSpecs(t, "Hydrate Suite")
}

var (
	ociImagePath string
	keep         bool
	helpers      *testhelpers.Helpers
)

var _ = BeforeSuite(func() {
	var (
		err     error
		present bool
	)

	ociImagePath, keep = os.LookupEnv("OCI_IMAGE_PATH")

	if !keep {
		ociImagePath, err = ioutil.TempDir("", "oci-image-path")
		logger := log.New(os.Stdout, "", 0)

		output, err := exec.Command("powershell", "-command", "[System.Environment]::OSVersion.Version.Build").CombinedOutput()
		Expect(err).NotTo(HaveOccurred())

		windowsBuild, err := strconv.Atoi(strings.TrimSpace(string(output)))
		Expect(err).NotTo(HaveOccurred())

		nanoserverTag := ""
		if windowsBuild == 16299 {
			nanoserverTag = "1709"
		} else {
			nanoserverTag = "1803"
		}

		imagefetcher.New(logger, ociImagePath, "microsoft/nanoserver", nanoserverTag, true).Run()
		Expect(err).ToNot(HaveOccurred())
	}

	debug, _ := strconv.ParseBool(os.Getenv("DEBUG"))

	grootBin, present := os.LookupEnv("GROOT_BINARY")
	Expect(present).To(BeTrue(), "GROOT_BINARY not set")

	grootImageStore, present := os.LookupEnv("GROOT_IMAGE_STORE")
	Expect(present).To(BeTrue(), "GROOT_IMAGE_STORE not set")

	wincBin, present := os.LookupEnv("WINC_BINARY")
	Expect(present).To(BeTrue(), "WINC_BINARY not set")

	diffBin, present := os.LookupEnv("DIFF_EXPORTER_BINARY")
	Expect(present).To(BeTrue(), "DIFF_EXPORTER_BINARY not set")

	hydrateBin, err := gexec.Build("code.cloudfoundry.org/hydrator/cmd/hydrate")
	Expect(err).NotTo(HaveOccurred())

	helpers = testhelpers.NewHelpers(wincBin, grootBin, grootImageStore, diffBin, hydrateBin, debug)
})

var _ = AfterSuite(func() {
	gexec.CleanupBuildArtifacts()

	if !keep {
		Expect(os.RemoveAll(ociImagePath)).To(Succeed())
	}
})
