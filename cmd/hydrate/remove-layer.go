package main

import (
	"errors"

	"code.cloudfoundry.org/hydrator/layermodifier"
	directory "code.cloudfoundry.org/hydrator/oci-directory"
	"github.com/urfave/cli"
)

var removeLayerCommand = cli.Command{
	Name:  "remove-layer",
	Usage: "removes the top layer from an existing image if it was added by hydrator",
	Description: `The remove-layer command removes the top layer from an existing OCI image
	if that layer was added by hydrator.
	Note that the OCI image must exist on disk and that the image will be modified
	in place`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "ociImage",
			Value: "",
			Usage: "Path to the image from which the layer will be removed",
		},
	},
	Action: func(context *cli.Context) error {
		if err := checkArgs(context, 0, exactArgs); err != nil {
			return err
		}
		ociImagePath := context.String("ociImage")

		if ociImagePath == "" {
			return errors.New("ERROR: Missing option -ociImage")
		}

		ociDirectory := directory.NewHandler(ociImagePath)
		layerModifier := layermodifier.New(ociDirectory)
		return layerModifier.RemoveHydratorLayer()
	},
}
