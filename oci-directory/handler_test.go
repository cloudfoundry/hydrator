package directory_test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	directory "code.cloudfoundry.org/hydrator/oci-directory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"

	"os"
)

var _ = Describe("Handler", func() {
	var (
		h            *directory.Handler
		layerDir     string
		ociImageDir  string
		layerTgzPath string
	)

	const (
		layerTgzContents = "xxxyyyzzz"
		layerTgzSHA256   = "cc6c955cadf2cc09442c0848ce8e165b8f9aa5974916de7186a9e1b6c4e7937e"
	)

	BeforeEach(func() {
		var err error
		ociImageDir, err = ioutil.TempDir("", "layermodifier.ociimagedir")
		Expect(err).NotTo(HaveOccurred())

		Expect(os.MkdirAll(filepath.Join(ociImageDir, "blobs", "sha256"), 0755)).To(Succeed())

		layerDir, err = ioutil.TempDir("", "oci-directory-layerdir")
		Expect(err).NotTo(HaveOccurred())

		layerTgzPath = filepath.Join(layerDir, "my-new-layer.tgz")
		Expect(ioutil.WriteFile(layerTgzPath, []byte(layerTgzContents), 0644)).To(Succeed())

		h = directory.NewHandler(ociImageDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(ociImageDir)).To(Succeed())
		Expect(os.RemoveAll(layerDir)).To(Succeed())
	})

	Describe("AddBlob", func() {
		Context("the oci image directory has a blobs/sha256 sub directory", func() {
			It("copies the layer.tgz into the blobs/sha256 sub directory, renaming to match the digest", func() {
				layerDescriptor := oci.Descriptor{
					Digest: digest.NewDigestFromEncoded("sha256", layerTgzSHA256),
				}

				Expect(h.AddBlob(layerTgzPath, layerDescriptor)).To(Succeed())
				contents, err := ioutil.ReadFile(filepath.Join(ociImageDir, "blobs", "sha256", layerTgzSHA256))
				Expect(err).NotTo(HaveOccurred())
				Expect(string(contents)).To(Equal(layerTgzContents))
			})
		})

		Context("the oci image directory does not have a blobs/sha256 sub directory", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(ociImageDir, "blobs", "sha256"))).To(Succeed())
			})

			It("returns a useful error", func() {
				err := h.AddBlob(layerTgzPath, oci.Descriptor{})
				Expect(err).To(MatchError(fmt.Sprintf("%s is not a valid OCI image: %s directory missing", ociImageDir, filepath.Join(ociImageDir, "blobs", "sha256"))))
			})
		})

		Context("the provided layer descriptor has an invalid digest", func() {
			It("returns a useful error", func() {
				err := h.AddBlob(layerTgzPath, oci.Descriptor{Digest: "notadigest"})
				Expect(err).To(Equal(digest.ErrDigestInvalidFormat))
			})
		})
	})

	Describe("RemoveTopBlob", func() {
		Context("the oci image directory has a blobs/sha256 sub directory", func() {
			Context("the layer exists in the directory", func() {
				BeforeEach(func() {
					layerDescriptor := oci.Descriptor{
						Digest: digest.NewDigestFromEncoded("sha256", layerTgzSHA256),
					}

					Expect(h.AddBlob(layerTgzPath, layerDescriptor)).To(Succeed())
				})

				It("removes the layer from the blobs/sha256 sub directory", func() {
					_, err := ioutil.ReadFile(filepath.Join(ociImageDir, "blobs", "sha256", layerTgzSHA256))
					Expect(err).ToNot(HaveOccurred())

					Expect(h.RemoveTopBlob(layerTgzSHA256)).To(Succeed())

					_, err = ioutil.ReadFile(filepath.Join(ociImageDir, "blobs", "sha256", layerTgzSHA256))
					Expect(err).To(HaveOccurred())
				})
			})
		})

		Context("the layer does not exist in the directory", func() {
			It("returns a useful error", func() {
				err := h.RemoveTopBlob(layerTgzSHA256)
				Expect(err).To(MatchError(fmt.Sprintf("%s does not contain layer: %s", ociImageDir, layerTgzSHA256)))
			})
		})

		Context("the oci image directory does not have a blobs/sha256 sub directory", func() {
			BeforeEach(func() {
				Expect(os.RemoveAll(filepath.Join(ociImageDir, "blobs", "sha256"))).To(Succeed())
			})

			It("returns a useful error", func() {
				err := h.RemoveTopBlob(layerTgzSHA256)
				Expect(err).To(MatchError(fmt.Sprintf("%s is not a valid OCI image: %s directory missing", ociImageDir, filepath.Join(ociImageDir, "blobs", "sha256"))))
			})
		})
	})

	Describe("ClearMetadata", func() {
		var (
			diffIds []digest.Digest
			layers  []oci.Descriptor
		)

		const (
			layer1 = "some-gzipped-data"
			layer2 = "more-gzipped"
			layer3 = "another-layer"

			layer1diffId = "dddddd"
			layer2diffId = "eeeeee"
			layer3diffId = "ffffff"
		)

		BeforeEach(func() {
			diffIds = []digest.Digest{
				digest.NewDigestFromEncoded("sha256", layer1diffId),
				digest.NewDigestFromEncoded("sha256", layer2diffId),
				digest.NewDigestFromEncoded("sha256", layer3diffId),
			}

			layers = []oci.Descriptor{
				{Digest: writeLayer(ociImageDir, layer1), MediaType: oci.MediaTypeImageLayerGzip},
				{Digest: writeLayer(ociImageDir, layer2), MediaType: oci.MediaTypeImageLayerGzip},
				{Digest: writeLayer(ociImageDir, layer3), MediaType: oci.MediaTypeImageLayerGzip},
			}

			h = directory.NewHandler(ociImageDir)
			Expect(h.WriteMetadata(layers, diffIds, false)).To(Succeed())
		})

		AfterEach(func() {
			Expect(os.RemoveAll(ociImageDir)).To(Succeed())
		})

		Context("all metadata files exist", func() {
			BeforeEach(func() {
				Expect(filepath.Join(ociImageDir, "oci-layout")).To(BeAnExistingFile())
				Expect(filepath.Join(ociImageDir, "index.json")).To(BeAnExistingFile())
				Expect(numBlobs(ociImageDir)).To(Equal(5)) // 3 layers, the manifest, and the config
			})

			It("deletes the metadata files", func() {
				Expect(h.ClearMetadata()).To(Succeed())
				Expect(filepath.Join(ociImageDir, "oci-layout")).NotTo(BeAnExistingFile())
				Expect(filepath.Join(ociImageDir, "index.json")).NotTo(BeAnExistingFile())
				Expect(numBlobs(ociImageDir)).To(Equal(3)) // only the layers are left
			})
		})

		Context("the index file does not exist", func() {
			BeforeEach(func() {
				Expect(os.Remove(filepath.Join(ociImageDir, "index.json"))).To(Succeed())
			})

			It("returns an error", func() {
				err := h.ClearMetadata()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("couldn't load index.json"))
			})
		})

		Context("the manifest file does not exist", func() {
			BeforeEach(func() {
				i := loadIndex(ociImageDir)
				Expect(os.Remove(filepath.Join(ociImageDir, "blobs", "sha256", i.Manifests[0].Digest.Encoded()))).To(Succeed())
			})

			It("returns an error", func() {
				err := h.ClearMetadata()
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("couldn't load manifest"))
			})
		})
	})
})

func numBlobs(ociImageDir string) int {
	infos, err := ioutil.ReadDir(filepath.Join(ociImageDir, "blobs", "sha256"))
	Expect(err).NotTo(HaveOccurred())
	return len(infos)
}
