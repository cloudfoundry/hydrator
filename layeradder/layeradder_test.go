package layeradder_test

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"

	"os"

	"code.cloudfoundry.org/hydrator/layeradder"
	fakes "code.cloudfoundry.org/hydrator/layeradder/fakes"
)

var _ = Describe("LayerAdder", func() {
	var (
		layerAdder       *layeradder.LayerAdder
		fakeOCIDirectory *fakes.OCIDirectory
		layerDir         string
		layerTgzPath     string
	)

	BeforeEach(func() {
		var err error
		layerDir, err = ioutil.TempDir("", "layeradder.layerdir")
		Expect(err).NotTo(HaveOccurred())

		layerTgzPath = filepath.Join(layerDir, "my-new-layer.tgz")

		manifest := oci.Manifest{
			Layers: []oci.Descriptor{
				{Digest: "layer1", Size: 1234, MediaType: oci.MediaTypeImageLayerGzip},
				{Digest: "layer2", Size: 6789, MediaType: oci.MediaTypeImageLayerGzip},
			},
		}
		config := oci.Image{
			RootFS: oci.RootFS{
				DiffIDs: []digest.Digest{
					digest.NewDigestFromEncoded(digest.SHA256, "abcd"),
					digest.NewDigestFromEncoded(digest.SHA256, "ef12"),
				},
			},
		}

		fakeOCIDirectory = &fakes.OCIDirectory{}
		fakeOCIDirectory.ReadMetadataReturns(manifest, config, nil)
		layerAdder = layeradder.New(fakeOCIDirectory)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(layerDir)).To(Succeed())
	})

	Describe("Add", func() {
		Context("the layer file is gzipped", func() {
			const (
				layerContents       = "some tar bytes"
				layerContentsSHA256 = "c5e8527cdf40bbdf7bb4b806ae96fee03355246be338b1fe3954e498248a44ca"
				gzippedSHA256       = "850e0c2747f004859b83554206506087e2e97f5bcabf316035c092e230ee0a60"
				gzippedSize         = 38
			)

			BeforeEach(func() {
				f, err := os.OpenFile(layerTgzPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
				Expect(err).NotTo(HaveOccurred())
				defer f.Close()

				gzw := gzip.NewWriter(f)
				defer gzw.Close()

				_, err = gzw.Write([]byte(layerContents))
				Expect(err).NotTo(HaveOccurred())
			})

			It("copies in the layer, and updates the OCI image metadata with the new layer", func() {
				Expect(layerAdder.Add(layerTgzPath)).To(Succeed())

				expectedDescriptor := oci.Descriptor{
					Digest:    digest.NewDigestFromEncoded(digest.SHA256, gzippedSHA256),
					MediaType: oci.MediaTypeImageLayerGzip,
					Size:      gzippedSize,
				}
				expectedDiffID := digest.NewDigestFromEncoded(digest.SHA256, layerContentsSHA256)

				Expect(fakeOCIDirectory.AddBlobCallCount()).To(Equal(1))
				p, desc := fakeOCIDirectory.AddBlobArgsForCall(0)
				Expect(p).To(Equal(layerTgzPath))
				Expect(desc).To(Equal(expectedDescriptor))

				Expect(fakeOCIDirectory.ReadMetadataCallCount()).To(Equal(1))

				Expect(fakeOCIDirectory.ClearMetadataCallCount()).To(Equal(1))

				Expect(fakeOCIDirectory.WriteMetadataCallCount()).To(Equal(1))
				newLayers, newDiffIDs := fakeOCIDirectory.WriteMetadataArgsForCall(0)

				expectedLayers := []oci.Descriptor{
					{Digest: "layer1", Size: 1234, MediaType: oci.MediaTypeImageLayerGzip},
					{Digest: "layer2", Size: 6789, MediaType: oci.MediaTypeImageLayerGzip},
					expectedDescriptor,
				}

				expectedDiffIDs := []digest.Digest{
					digest.NewDigestFromEncoded(digest.SHA256, "abcd"),
					digest.NewDigestFromEncoded(digest.SHA256, "ef12"),
					expectedDiffID,
				}

				Expect(newLayers).To(Equal(expectedLayers))
				Expect(newDiffIDs).To(Equal(expectedDiffIDs))
			})

			Context("Adding the blob fails", func() {
				BeforeEach(func() {
					fakeOCIDirectory.AddBlobReturns(errors.New("failed to add blob"))
				})

				It("returns the error", func() {
					Expect(layerAdder.Add(layerTgzPath)).To(MatchError("failed to add blob"))

					Expect(fakeOCIDirectory.ReadMetadataCallCount()).To(Equal(0))
					Expect(fakeOCIDirectory.ClearMetadataCallCount()).To(Equal(0))
					Expect(fakeOCIDirectory.WriteMetadataCallCount()).To(Equal(0))
				})
			})

			Context("Reading the metadata fails", func() {
				BeforeEach(func() {
					fakeOCIDirectory.ReadMetadataReturns(oci.Manifest{}, oci.Image{}, errors.New("failed to read metadata"))
				})

				It("returns the error", func() {
					Expect(layerAdder.Add(layerTgzPath)).To(MatchError("failed to read metadata"))

					Expect(fakeOCIDirectory.ClearMetadataCallCount()).To(Equal(0))
					Expect(fakeOCIDirectory.WriteMetadataCallCount()).To(Equal(0))
				})
			})

			Context("Clearing the metadata fails", func() {
				BeforeEach(func() {
					fakeOCIDirectory.ClearMetadataReturns(errors.New("failed to clear metadata"))
				})

				It("returns the error", func() {
					Expect(layerAdder.Add(layerTgzPath)).To(MatchError("failed to clear metadata"))

					Expect(fakeOCIDirectory.WriteMetadataCallCount()).To(Equal(0))
				})
			})

			Context("Writing the new metadata fails", func() {
				BeforeEach(func() {
					fakeOCIDirectory.WriteMetadataReturns(errors.New("failed to write metadata"))
				})

				It("returns the error", func() {
					Expect(layerAdder.Add(layerTgzPath)).To(MatchError("failed to write metadata"))
				})
			})
		})

		Context("the layer file is not gzipped", func() {
			BeforeEach(func() {
				Expect(ioutil.WriteFile(layerTgzPath, []byte("not gzipped data"), 0644)).To(Succeed())
			})

			It("returns an error", func() {
				err := layerAdder.Add(layerTgzPath)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(fmt.Sprintf("invalid layer %s: not gzipped", layerTgzPath)))
			})
		})

		Context("the layer file does not exist", func() {
			It("returns an error", func() {
				err := layerAdder.Add("invalid/layer/path")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
