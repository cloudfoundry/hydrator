package main

import (
	"errors"

	"code.cloudfoundry.org/hydrator/layeradder"
	directory "code.cloudfoundry.org/hydrator/oci-directory"
	"github.com/urfave/cli"
)

var addLayerCommand = cli.Command{
	Name:  "add-layer",
	Usage: "adds a layer to an existing image",
	Description: `The add-layer command adds a layer to an existing OCI image.
	Note that the OCI image must exist on disk and that the image will be modified
	in place`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ociImage",
			Value: "",
			Usage: "Path to the image where the layer will be added",
		},
		cli.StringFlag{
			Name:  "layer",
			Value: "",
			Usage: "Path to .tgz file containing the layer to be added to the image",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 0, exactArgs); err != nil {
			return err
		}
		layerPath := context.String("layer")
		ociImagePath := context.String("ociImage")

		if layerPath == "" {
			return errors.New("ERROR: Missing option -layer")
		}
		if ociImagePath == "" {
			return errors.New("ERROR: Missing option -ociImage")
		}

		ociDirectory := directory.NewHandler(ociImagePath)
		layerAdder := layeradder.New(ociDirectory)
		return layerAdder.Add(layerPath)
	},
}
