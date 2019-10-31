package imagefetcher

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"code.cloudfoundry.org/hydrator/compress"
	"code.cloudfoundry.org/hydrator/downloader"
	directory "code.cloudfoundry.org/hydrator/oci-directory"
	"code.cloudfoundry.org/hydrator/registry"
)

/*
 * These are for users who do not have their own registry server, or
 * Authorization server or both. They can use docker's hosted services.
 */
const (
	DefaultRegistryServerURL = "https://registry.hub.docker.com"
	DefaultAuthServerURL     = "https://auth.docker.io"
	DefaultAuthServiceName   = "registry.docker.io"
)

type RegistryParams struct {
	RegistryServerURL string
	AuthServerURL     string
	AuthServiceName   string
}

type ImageFetcher struct {
	logger         *log.Logger
	outDir         string
	imageName      string
	imageTag       string
	registry       string
	registryParams *RegistryParams
	noTarball      bool
}

func New(logger *log.Logger, outDir, imageName, imageTag, registry,
	authServer, authServiceName string, noTarball bool) *ImageFetcher {

	ifetcher := &ImageFetcher{
		logger:         logger,
		outDir:         outDir,
		imageName:      imageName,
		imageTag:       imageTag,
		registryParams: nil,
		noTarball:      noTarball,
	}
	ifetcher.SetRegistryParams(registry, authServer, authServiceName)
	return ifetcher
}

func (i *ImageFetcher) GetRegistryParams() *RegistryParams {
	return i.registryParams
}

func (i *ImageFetcher) SetRegistryParams(registry, authServer, authServiceName string) {
	i.registryParams = &RegistryParams{
		RegistryServerURL: DefaultRegistryServerURL,
		AuthServerURL:     DefaultAuthServerURL,
		AuthServiceName:   DefaultAuthServiceName,
	}
	if registry != "" {
		i.registryParams.RegistryServerURL = registry
	}
	if authServer != "" {
		i.registryParams.AuthServerURL = authServer
	}
	if authServiceName != "" {
		i.registryParams.AuthServiceName = authServiceName
	}
}

func (i *ImageFetcher) Run() error {
	var imageDownloadDir string

	if err := os.MkdirAll(i.outDir, 0755); err != nil {
		return errors.New("ERROR: Could not create output directory")
	}

	if i.noTarball {
		imageDownloadDir = i.outDir
	} else {
		tempDir, err := ioutil.TempDir("", "hydrate")
		if err != nil {
			return fmt.Errorf("Could not create tmp dir: %s", tempDir)
		}
		defer os.RemoveAll(tempDir)

		imageDownloadDir = tempDir
	}

	blobDownloadDir := filepath.Join(imageDownloadDir, "blobs", "sha256")
	if err := os.MkdirAll(blobDownloadDir, 0755); err != nil {
		return err
	}

	r := registry.New(i.registryParams.AuthServerURL,
		i.registryParams.AuthServiceName,
		i.registryParams.RegistryServerURL,
		i.imageName,
		i.imageTag,
	)

	d := downloader.New(i.logger, blobDownloadDir, r)

	i.logger.Printf("\nDownloading image: %s with tag: %s from registry: %s\n",
		i.imageName, i.imageTag, i.registry)
	layers, diffIds, err := d.Run()
	if err != nil {
		return fmt.Errorf("Failed downloading image: %s with tag: %s from registry: %s - %s", i.imageName, i.imageTag, i.registry, err)
	}

	handler := directory.NewHandler(imageDownloadDir)
	if err := handler.WriteMetadata(layers, diffIds, false); err != nil {
		return err
	}
	i.logger.Printf("\nAll layers downloaded.\n")

	if !i.noTarball {
		nameParts := strings.Split(i.imageName, "/")
		if len(nameParts) != 2 {
			return errors.New("Invalid image name")
		}
		outFile := filepath.Join(i.outDir, fmt.Sprintf("%s-%s.tgz", nameParts[1], i.imageTag))

		i.logger.Printf("Writing %s...\n", outFile)

		c := compress.New()
		if err := c.WriteTgz(imageDownloadDir, outFile); err != nil {
			return err
		}

		i.logger.Println("Done.")
	}

	return nil
}
