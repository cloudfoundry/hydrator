package main

import (
	"fmt"
	"os"

	"github.com/urfave/cli"
)

const (
	exactArgs = iota
	minArgs
	maxArgs
)

func main() {
	app := cli.NewApp()
	app.Name = "hydrate.exe"

	app.Commands = []cli.Command{
		downloadCommand,
		addLayerCommand,
		removeLayerCommand,
	}

	if err := app.Run(os.Args); err != nil {
		fatal(err)
	}
}

func checkArgs(context *cli.Context, expected, checkType int) error {
	var err error
	cmdName := context.Command.Name
	switch checkType {
	case exactArgs:
		if context.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case minArgs:
		if context.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case maxArgs:
		if context.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}

	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		_ = cli.ShowCommandHelp(context, cmdName)
		return err
	}
	return nil
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
