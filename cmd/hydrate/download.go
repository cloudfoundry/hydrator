package main

import (
	"errors"
	"log"
	"os"

	"code.cloudfoundry.org/hydrator/imagefetcher"
	"github.com/urfave/cli"
)

var downloadCommand = cli.Command{
	Name:  "download",
	Usage: "downloads an image",
	Description: `The download command downloads an image from registry.hub.docker.com.
	The downloaded image is formatted according to the OCI Image Format Specification`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "outputDir",
			Value: os.TempDir(),
			Usage: "Output directory for downloaded image",
		},
		cli.StringFlag{
			Name:  "image",
			Value: "",
			Usage: "Name of the image to download",
		},
		cli.StringFlag{
			Name:  "registryServer",
			Value: "",
			Usage: "Registry server url that stores docker images (defaults to dockerhub url)",
		},
		cli.StringFlag{
			Name:  "authServer",
			Value: "",
			Usage: "Authorization server url (defaults to dockerhub url)",
		},
		cli.StringFlag{
			Name:  "authServiceName",
			Value: "",
			Usage: "Name of the authorization service (defaults to docker service)",
		},
		// TODO implement this in imagefetcher
		// cli.BoolFlag{
		// 	Name:  "no-auth",
		// 	Usage: "Do not use authorization to download",
		// },
		cli.StringFlag{
			Name:  "tag",
			Value: "latest",
			Usage: "Image tag to download",
		},
		cli.BoolFlag{
			Name:  "noTarball",
			Usage: "Do not output image as a tarball",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 0, exactArgs); err != nil {
			return err
		}

		logger := log.New(os.Stdout, "", 0)

		imageName := context.String("image")
		if imageName == "" {
			return errors.New("ERROR: No image name provided")
		}

		return imagefetcher.New(logger,
			context.String("outputDir"),
			imageName,
			context.String("tag"),
			context.String("registryServer"),
			context.String("authServer"),
			context.String("authServiceName"),
			context.Bool("noTarball"),
		).Run()
	},
}
