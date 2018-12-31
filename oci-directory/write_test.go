package directory_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	directory "code.cloudfoundry.org/hydrator/oci-directory"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	digest "github.com/opencontainers/go-digest"
	specs "github.com/opencontainers/image-spec/specs-go"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("WriteMetadata", func() {
	var (
		h          *directory.Handler
		layers     []oci.Descriptor
		diffIds    []digest.Digest
		layerAdded bool
		outDir     string
	)

	BeforeEach(func() {
		var err error
		outDir, err = ioutil.TempDir("", "oci-directory.write.test")
		Expect(err).NotTo(HaveOccurred())

		layers = []oci.Descriptor{
			{Digest: "layer1", Size: 1234, MediaType: oci.MediaTypeImageLayerGzip},
			{Digest: "layer2", Size: 6789, MediaType: oci.MediaTypeImageLayerGzip},
		}

		diffIds = []digest.Digest{digest.NewDigestFromEncoded(digest.SHA256, "aaaaaa"), digest.NewDigestFromEncoded(digest.SHA256, "bbbbbb")}

		layerAdded = false

		h = directory.NewHandler(outDir)
	})

	AfterEach(func() {
		Expect(os.RemoveAll(outDir)).To(Succeed())
	})

	It("writes a valid oci layout file", func() {
		Expect(h.WriteMetadata(layers, diffIds, layerAdded)).To(Succeed())

		var il oci.ImageLayout
		content, err := ioutil.ReadFile(filepath.Join(outDir, "oci-layout"))
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(content, &il)).To(Succeed())
		Expect(il.Version).To(Equal(specs.Version))
	})

	It("writes a valid index.json file", func() {
		Expect(h.WriteMetadata(layers, diffIds, layerAdded)).To(Succeed())

		ii := loadIndex(outDir)
		Expect(ii.SchemaVersion).To(Equal(2))
		Expect(len(ii.Manifests)).To(Equal(1))

		manifestDescriptor := ii.Manifests[0]
		Expect(manifestDescriptor.MediaType).To(Equal(oci.MediaTypeImageManifest))
		Expect(*manifestDescriptor.Platform).To(Equal(oci.Platform{OS: "windows", Architecture: "amd64"}))

		manifestAlgorithm := manifestDescriptor.Digest.Algorithm()
		Expect(manifestAlgorithm).To(Equal(digest.SHA256))

		manifestSha := manifestDescriptor.Digest.Encoded()
		manifestFile := filepath.Join(outDir, "blobs", manifestAlgorithm.String(), manifestSha)

		fi, err := os.Stat(manifestFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(fi.Size()).To(Equal(manifestDescriptor.Size))
		Expect(sha256Sum(manifestFile)).To(Equal(manifestSha))
	})

	Context("When writing a valid manifest file", func() {
		It("generates an image config and sets the layerAdded annotation to false", func() {
			Expect(h.WriteMetadata(layers, diffIds, layerAdded)).To(Succeed())

			im := loadManifest(outDir)

			Expect(im.Layers).To(ConsistOf(layers))
			Expect(im.SchemaVersion).To(Equal(2))
			Expect(im.Annotations).NotTo(HaveKey("hydrator.layerAdded"))

			configFile := filepath.Join(outDir, "blobs", im.Config.Digest.Algorithm().String(), im.Config.Digest.Encoded())
			fi, err := os.Stat(configFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.Size()).To(Equal(im.Config.Size))
			Expect(sha256Sum(configFile)).To(Equal(im.Config.Digest.Encoded()))
		})

		It("generates an image config and sets the layerAdded annotation to true", func() {
			layerAdded = true
			Expect(h.WriteMetadata(layers, diffIds, layerAdded)).To(Succeed())

			im := loadManifest(outDir)

			Expect(im.Layers).To(ConsistOf(layers))
			Expect(im.SchemaVersion).To(Equal(2))
			Expect(im.Annotations).To(HaveKeyWithValue("hydrator.layerAdded", "true"))

			configFile := filepath.Join(outDir, "blobs", im.Config.Digest.Algorithm().String(), im.Config.Digest.Encoded())
			fi, err := os.Stat(configFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(fi.Size()).To(Equal(im.Config.Size))
			Expect(sha256Sum(configFile)).To(Equal(im.Config.Digest.Encoded()))
		})
	})

	It("writes a valid image config file", func() {
		Expect(h.WriteMetadata(layers, diffIds, layerAdded)).To(Succeed())

		ic := loadConfig(outDir)

		Expect(ic.Architecture).To(Equal("amd64"))
		Expect(ic.OS).To(Equal("windows"))
		expectedRootFS := oci.RootFS{Type: "layers", DiffIDs: diffIds}
		Expect(ic.RootFS).To(Equal(expectedRootFS))
	})
})

func loadIndex(outDir string) oci.Index {
	var ii oci.Index
	content, err := ioutil.ReadFile(filepath.Join(outDir, "index.json"))
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(content, &ii)).To(Succeed())

	return ii
}

func loadManifest(outDir string) oci.Manifest {
	ii := loadIndex(outDir)

	manifestDescriptor := ii.Manifests[0]
	manifestFile := filepath.Join(outDir, "blobs", manifestDescriptor.Digest.Algorithm().String(), manifestDescriptor.Digest.Encoded())

	content, err := ioutil.ReadFile(manifestFile)
	Expect(err).NotTo(HaveOccurred())

	var im oci.Manifest
	Expect(json.Unmarshal(content, &im)).To(Succeed())
	return im
}

func loadConfig(outDir string) oci.Image {
	im := loadManifest(outDir)

	configFile := filepath.Join(outDir, "blobs", im.Config.Digest.Algorithm().String(), im.Config.Digest.Encoded())

	content, err := ioutil.ReadFile(configFile)
	Expect(err).NotTo(HaveOccurred())

	var ic oci.Image
	Expect(json.Unmarshal(content, &ic)).To(Succeed())
	return ic
}

func sha256Sum(file string) string {
	Expect(file).To(BeAnExistingFile())
	f, err := os.Open(file)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%x", h.Sum(nil))
}
