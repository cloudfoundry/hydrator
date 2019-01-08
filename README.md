# hydrator

The `hydrator` downloads Docker images and lays them out on disk in [OCI image format](https://github.com/opencontainers/image-spec).

## usage
```
$ ./hydrate --help
NAME:
   hydrate.exe - A new cli application

USAGE:
   hydrate [global options] command [command options] [arguments...]

VERSION:
   0.0.0

COMMANDS:
     download      downloads an image
     add-layer     adds a layer to an existing image
     remove-layer  removes the top layer from an existing image
     help, h       Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --help, -h     show help
   --version, -v  print the version
```
