$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

ginkgo.exe -r --race -keep-going --randomize-suites
if ($LastExitCode -ne 0) {
  Exit 1
}
