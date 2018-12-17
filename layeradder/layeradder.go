package layeradder

import (
	"fmt"
	"os"

	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/v1util"
	digest "github.com/opencontainers/go-digest"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter -o fakes/oci_directory.go --fake-name OCIDirectory . OCIDirectory
type OCIDirectory interface {
	AddBlob(srcPath string, blobDescriptor oci.Descriptor) error
	ClearMetadata() error
	ReadMetadata() (oci.Manifest, oci.Image, error)
	WriteMetadata(layers []oci.Descriptor, diffIds []digest.Digest) error
}

type LayerAdder struct {
	ociDirectory OCIDirectory
}

func New(ociDirectory OCIDirectory) *LayerAdder {
	return &LayerAdder{
		ociDirectory: ociDirectory,
	}
}

func (l *LayerAdder) Add(layerTgzPath string) error {
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
	return l.ociDirectory.WriteMetadata(newLayers, newDiffIDs)
}

func (l *LayerAdder) getLayerDescriptor(layerTgzPath string) (oci.Descriptor, digest.Digest, error) {
	layerfd, err := os.Open(layerTgzPath)
	if err != nil {
		return oci.Descriptor{}, "", err
	}
	defer layerfd.Close()

	compressed, err := v1util.IsGzipped(layerfd)
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
