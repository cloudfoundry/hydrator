package downloader

import (
	"fmt"
	"io"
	"sync"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate counterfeiter . Registry
type Registry interface {
	Manifest() (v1.Manifest, error)
	Config(v1.Descriptor) (v1.Image, error)
	DownloadLayer(v1.Descriptor, string) error
}

type Downloader struct {
	downloadDir string
	registry    Registry
	logger      io.Writer
}

func New(downloadDir string, registry Registry, logger io.Writer) *Downloader {
	d := &Downloader{
		downloadDir: downloadDir,
		registry:    registry,
		logger:      logger,
	}
	return d
}

func (d *Downloader) Run() ([]v1.Descriptor, []digest.Digest, error) {
	registryManifest, err := d.registry.Manifest()
	if err != nil {
		return nil, nil, err
	}

	registryConfig, err := d.registry.Config(registryManifest.Config)
	if err != nil {
		return nil, nil, err
	}

	if registryConfig.OS != "windows" {
		return nil, nil, fmt.Errorf("invalid container OS: %s", registryConfig.OS)
	}
	if registryConfig.Architecture != "amd64" {
		return nil, nil, fmt.Errorf("invalid container arch: %s", registryConfig.Architecture)
	}

	totalLayers := len(registryManifest.Layers)
	diffIds := registryConfig.RootFS.DiffIDs

	if totalLayers != len(diffIds) {
		return nil, nil, fmt.Errorf("mismatch: %d layers, %d diffIds", totalLayers, len(diffIds))
	}

	fmt.Fprintf(d.logger, "Downloading %d layers...\n", totalLayers)
	wg := sync.WaitGroup{}
	errChan := make(chan error, 1)

	downloadedLayers := []v1.Descriptor{}

	for i, layer := range registryManifest.Layers {
		l := layer
		diffId := diffIds[i]

		ociLayer := v1.Descriptor{
			MediaType: v1.MediaTypeImageLayerGzip,
			Size:      l.Size,
			Digest:    l.Digest,
		}

		downloadedLayers = append(downloadedLayers, ociLayer)

		wg.Add(1)
		go func() {
			fmt.Fprintf(d.logger, "Layer diffID: %.8s, sha256: %.8s begin\n", diffId.Encoded(), l.Digest.Encoded())
			defer wg.Done()
			attempt := 0
			for {
				attempt += 1
				err := d.registry.DownloadLayer(l, d.downloadDir)
				if err != nil {
					fmt.Fprintf(d.logger, "Attempt %d failed downloading layer with diffID: %.8s, sha256: %.8s: %s\n", attempt, diffId.Encoded(), l.Digest.Encoded(), err)

					if attempt >= 5 {
						errChan <- &MaxLayerDownloadRetriesError{DiffID: diffId.Encoded(), SHA: l.Digest.Encoded()}
						break
					}

					time.Sleep(time.Duration(attempt) * time.Second)
					continue
				}

				fmt.Fprintf(d.logger, "Layer diffID: %.8s, sha256: %.8s end\n", diffId.Encoded(), l.Digest.Encoded())
				break
			}
		}()
	}

	wgEmpty := make(chan interface{}, 1)
	go func() {
		wg.Wait()
		wgEmpty <- nil
	}()

	select {
	case <-wgEmpty:
	case downloadErr := <-errChan:
		return nil, nil, downloadErr
	}

	return downloadedLayers, diffIds, nil
}
