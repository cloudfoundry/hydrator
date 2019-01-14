# hydrator

The `hydrator` downloads Docker images and lays them out on disk in [OCI image format](https://github.com/opencontainers/image-spec). It can also be used to add and remove layers to/from the OCI image. (see `add-layer`, `remove-layer` option)

## Building

Make sure `GOPATH` is set. Then run:

```
go build .\cmd\hydrate
```

It generates a `hydrate.exe` in the current directory.

## Usage

```
hydrate.exe [global options] command [command options] [arguments...]
```

#### Example

```
hydrate.exe download -image cloudfoundry/windows2016fs -tag 1803 -outputDir C:\hydratorOutput -noTarball
```

Use `hydrate --help` to show detailed usage.

## Testing

#### Requirements

* [groot](https://github.com/cloudfoundry/groot-windows)
* [winc](https://github.com/cloudfoundry/winc)
* [diff-exporter](https://github.com/cloudfoundry-incubator/diff-exporter)

To run the entire suite of tests, do `ginkgo -r .`.

The tests require the following env variables to be set:

* `GROOT_BINARY`

* `GROOT_IMAGE_STORE`

* `WINC_BINARY`

* `DIFF_EXPORTER_BINARY`
