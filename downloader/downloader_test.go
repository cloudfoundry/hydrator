package downloader_test

import (
	"bytes"
	"errors"
	"io"
	"log"

	"code.cloudfoundry.org/hydrator/downloader"
	"code.cloudfoundry.org/hydrator/downloader/fakes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Downloader", func() {
	const (
		downloadDir = "some-directory"
	)

	var (
		sourceLayers   []v1.Descriptor
		sourceDiffIds  []digest.Digest
		sourceConfig   v1.Image
		manifestConfig v1.Descriptor
		manifest       v1.Manifest
		registry       *fakes.Registry
		d              *downloader.Downloader
		logBuffer      *bytes.Buffer
	)

	BeforeEach(func() {
		sourceLayers = []v1.Descriptor{
			{Digest: digest.NewDigestFromEncoded(digest.SHA256, "layer1"), Size: 1234},
			{Digest: digest.NewDigestFromEncoded(digest.SHA256, "layer2"), Size: 6789},
		}
		manifestConfig = v1.Descriptor{Digest: "config", Size: 7777}
		manifest = v1.Manifest{Layers: sourceLayers, Config: manifestConfig}

		sourceDiffIds = []digest.Digest{
			digest.NewDigestFromEncoded(digest.SHA256, "aaaaaa"),
			digest.NewDigestFromEncoded(digest.SHA256, "bbbbbb"),
		}
		sourceConfig = v1.Image{
			OS:           "windows",
			Architecture: "amd64",
			RootFS:       v1.RootFS{Type: "layers", DiffIDs: sourceDiffIds},
		}

		registry = &fakes.Registry{}

		registry.ManifestReturnsOnCall(0, manifest, nil)
		registry.ConfigReturnsOnCall(0, sourceConfig, nil)

		logBuffer = new(bytes.Buffer)
		d = downloader.New(log.New(io.MultiWriter(GinkgoWriter, logBuffer), "", 0), downloadDir, registry)
	})

	Describe("Run", func() {
		It("Uses the manifest to download the config, all the layers and returns the proper descriptors + diffIds", func() {
			layers, diffIds, err := d.Run()
			Expect(err).NotTo(HaveOccurred())

			Expect(layers[0].Digest).To(Equal(digest.Digest("sha256:layer1")))
			Expect(layers[0].Size).To(Equal(int64(1234)))
			Expect(layers[0].MediaType).To(Equal(v1.MediaTypeImageLayerGzip))

			Expect(layers[1].Digest).To(Equal(digest.Digest("sha256:layer2")))
			Expect(layers[1].Size).To(Equal(int64(6789)))
			Expect(layers[1].MediaType).To(Equal(v1.MediaTypeImageLayerGzip))

			Expect(diffIds).To(ConsistOf(sourceDiffIds))

			Expect(registry.ManifestCallCount()).To(Equal(1))
			Expect(registry.ConfigCallCount()).To(Equal(1))
			Expect(registry.ConfigArgsForCall(0)).To(Equal(manifestConfig))

			Expect(registry.DownloadLayerCallCount()).To(Equal(2))
			l1, dir := registry.DownloadLayerArgsForCall(0)
			Expect(dir).To(Equal("some-directory"))

			l2, dir := registry.DownloadLayerArgsForCall(1)
			Expect(dir).To(Equal("some-directory"))

			Expect([]v1.Descriptor{l1, l2}).To(ConsistOf(sourceLayers))
		})

		Context("downloading a layer fails inconsistently", func() {
			BeforeEach(func() {
				registry.DownloadLayerReturnsOnCall(0, errors.New("couldn't download layer error 1"))
				registry.DownloadLayerReturnsOnCall(1, errors.New("couldn't download layer error 2"))
				registry.DownloadLayerReturnsOnCall(2, errors.New("couldn't download layer error 3"))
			})

			It("retries and succeeds", func() {
				layers, _, err := d.Run()
				Expect(err).NotTo(HaveOccurred())

				Expect(layers[0].Digest).To(Equal(digest.Digest("sha256:layer1")))
				Expect(registry.DownloadLayerCallCount()).To(Equal(5))
			})

			It("logs the retries", func() {
				layers, _, err := d.Run()
				Expect(err).NotTo(HaveOccurred())

				Expect(layers[0].Digest).To(Equal(digest.Digest("sha256:layer1")))

				Expect(logBuffer.String()).To(MatchRegexp("Attempt [0-9] failed downloading layer with diffID: [[:alnum:]]+, sha256: [[:alnum:]]+: couldn't download layer error 1\n"))
				Expect(logBuffer.String()).To(MatchRegexp("Attempt [0-9] failed downloading layer with diffID: [[:alnum:]]+, sha256: [[:alnum:]]+: couldn't download layer error 2\n"))
				Expect(logBuffer.String()).To(MatchRegexp("Attempt [0-9] failed downloading layer with diffID: [[:alnum:]]+, sha256: [[:alnum:]]+: couldn't download layer error 3\n"))
			})
		})

		Context("downloading a layer fails every time", func() {
			BeforeEach(func() {
				registry.DownloadLayerReturns(errors.New("couldn't download layer"))
			})

			It("retries each layer the max number of times and then returns a descriptive error", func() {
				_, _, err := d.Run()
				Expect(err).To(BeAssignableToTypeOf(&downloader.MaxLayerDownloadRetriesError{}))
				Expect(registry.DownloadLayerCallCount()).To(BeNumerically(">=", 5))
			})
		})
	})

	Context("getting the manifest fails", func() {
		BeforeEach(func() {
			registry.ManifestReturnsOnCall(0, v1.Manifest{}, errors.New("couldn't get manifest"))
		})

		It("returns an error", func() {
			_, _, err := d.Run()
			Expect(err.Error()).To(Equal("couldn't get manifest"))
			Expect(registry.DownloadLayerCallCount()).To(Equal(0))
		})
	})

	Context("manifest and config have different # of layers", func() {
		BeforeEach(func() {
			sourceDiffIds = []digest.Digest{
				digest.NewDigestFromEncoded(digest.SHA256, "aaaaaa"),
			}
			sourceConfig = v1.Image{
				OS:           "windows",
				Architecture: "amd64",
				RootFS:       v1.RootFS{Type: "layers", DiffIDs: sourceDiffIds},
			}

			registry.ConfigReturnsOnCall(0, sourceConfig, nil)
		})

		It("returns an error", func() {
			_, _, err := d.Run()
			Expect(err.Error()).To(Equal("mismatch: 2 layers, 1 diffIds"))
			Expect(registry.DownloadLayerCallCount()).To(Equal(0))
		})
	})

	Context("config has invalid OS", func() {
		BeforeEach(func() {
			sourceConfig = v1.Image{
				OS: "linux",
			}

			registry.ConfigReturnsOnCall(0, sourceConfig, nil)
		})

		It("returns an error", func() {
			_, _, err := d.Run()
			Expect(err.Error()).To(Equal("invalid container OS: linux"))
			Expect(registry.DownloadLayerCallCount()).To(Equal(0))
		})
	})

	Context("config has invalid architecture", func() {
		BeforeEach(func() {
			sourceConfig = v1.Image{
				OS:           "windows",
				Architecture: "ppc64",
			}

			registry.ConfigReturnsOnCall(0, sourceConfig, nil)
		})

		It("returns an error", func() {
			_, _, err := d.Run()
			Expect(err.Error()).To(Equal("invalid container arch: ppc64"))
			Expect(registry.DownloadLayerCallCount()).To(Equal(0))
		})
	})
})
