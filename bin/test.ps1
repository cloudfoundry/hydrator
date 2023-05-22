$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

ginkgo.exe -r --race -keep-going --randomize-suites
if ($LASTEXITCODE -ne 0) {
    throw "tests failed"
}
