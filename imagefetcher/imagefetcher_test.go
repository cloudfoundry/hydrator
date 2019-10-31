package imagefetcher_test

import (
	"log"
	"os"

	. "code.cloudfoundry.org/hydrator/imagefetcher"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ImageFetcher", func() {

	Context("when the imagefetcher recieves no registry flags", func() {
		var fetcher *ImageFetcher

		BeforeEach(func() {
			logger := log.New(os.Stdout, "", 0)
			fetcher = New(logger, "some-dir", "some-image", "some-tag",
				"", "", "", true)
		})

		It("successfully uses docker's registry parameters", func() {
			rparams := fetcher.GetRegistryParams()
			Expect(rparams).NotTo(BeNil())
			Expect(rparams.RegistryServerURL).To(Equal(DefaultRegistryServerURL))
			Expect(rparams.AuthServerURL).To(Equal(DefaultAuthServerURL))
			Expect(rparams.AuthServiceName).To(Equal(DefaultAuthServiceName))
		})

	})

	Context("when the imagefetcher receives registry flags", func() {
		var fetcher *ImageFetcher
		var myRegistryServer string
		var myAuthServer string
		var myAuthServiceName string

		BeforeEach(func() {
			logger := log.New(os.Stdout, "", 0)
			myRegistryServer = "http://server.whatever"
			myAuthServer = "https://auth.whatever"
			myAuthServiceName = "registry.whatever"
			fetcher = New(logger, "some-dir", "some-image", "some-tag",
				myRegistryServer, myAuthServer, myAuthServiceName, true)
		})

		It("successfully uses the given registry parameters", func() {
			rparams := fetcher.GetRegistryParams()
			Expect(rparams).NotTo(BeNil())
			Expect(rparams.RegistryServerURL).To(Equal(myRegistryServer))
			Expect(rparams.AuthServerURL).To(Equal(myAuthServer))
			Expect(rparams.AuthServiceName).To(Equal(myAuthServiceName))
		})
	})
})
