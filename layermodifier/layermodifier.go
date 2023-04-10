package layermodifier

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	digest "github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter -o fakes/oci_directory.go --fake-name OCIDirectory . OCIDirectory
type OCIDirectory interface {
	AddBlob(srcPath string, blobDescriptor oci.Descriptor) error
	RemoveTopBlob(sha256 string) error
	ClearMetadata() error
	ReadMetadata() (oci.Manifest, oci.Image, error)
	WriteMetadata(layers []oci.Descriptor, diffIds []digest.Digest, layerAdded bool) error
}

type LayerModifier struct {
	ociDirectory OCIDirectory
}

func New(ociDirectory OCIDirectory) *LayerModifier {
	return &LayerModifier{
		ociDirectory: ociDirectory,
	}
}

func (l *LayerModifier) AddLayer(layerTgzPath string) error {
	descriptor, diffId, err := l.getLayerDescriptor(layerTgzPath)
	if err != nil {
		return err
	}

	if err := l.ociDirectory.AddBlob(layerTgzPath, descriptor); err != nil {
		return err
	}

	manifest, config, err := l.ociDirectory.ReadMetadata()
	if err != nil {
		return err
	}

	if err := l.ociDirectory.ClearMetadata(); err != nil {
		return err
	}

	newLayers := append(manifest.Layers, descriptor)
	newDiffIDs := append(config.RootFS.DiffIDs, diffId)
	layerAdded := true
	return l.ociDirectory.WriteMetadata(newLayers, newDiffIDs, layerAdded)
}

func (l *LayerModifier) RemoveHydratorLayer() error {
	manifest, config, err := l.ociDirectory.ReadMetadata()
	if err != nil {
		return err
	}

	if _, ok := manifest.Annotations["hydrator.layerAdded"]; !ok {
		return nil
	}

	if err := l.ociDirectory.ClearMetadata(); err != nil {
		return err
	}

	lastLayer := manifest.Layers[len(manifest.Layers)-1]
	layerDigest := (strings.Split(string(lastLayer.Digest), ":"))[1] //lastLayer.Digest = "sha256:LAYER_SHA"
	if err := l.ociDirectory.RemoveTopBlob(layerDigest); err != nil {
		return err
	}

	newLayers := manifest.Layers[:len(manifest.Layers)-1]
	newDiffIDs := config.RootFS.DiffIDs[:len(config.RootFS.DiffIDs)-1]
	layerAdded := false

	return l.ociDirectory.WriteMetadata(newLayers, newDiffIDs, layerAdded)
}

func (l *LayerModifier) getLayerDescriptor(layerTgzPath string) (oci.Descriptor, digest.Digest, error) {
	layerfd, err := os.Open(layerTgzPath)
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	defer layerfd.Close()

	compressed, err := isGzipped(layerfd)
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	if !compressed {
		return oci.Descriptor{}, "", fmt.Errorf("invalid layer %s: not gzipped", layerTgzPath)
	}

	layer, err := tarball.LayerFromFile(layerTgzPath)
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	layerDigest, err := layer.Digest()
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	size, err := layer.Size()
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	diffID, err := layer.DiffID()
	if err != nil {
		return oci.Descriptor{}, "", err
	}

	return oci.Descriptor{
		Digest:    digest.Digest(layerDigest.String()),
		MediaType: oci.MediaTypeImageLayerGzip,
		Size:      size,
	}, digest.Digest(diffID.String()), nil
}

var gzipHeader = []byte{'\x1f', '\x8b'}

func isGzipped(file *os.File) (bool, error) {
	header := make([]byte, 2)
	n, err := file.Read(header)
	if n == 0 && err == io.EOF {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return bytes.Equal(header, gzipHeader), nil
}
