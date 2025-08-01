$ErrorActionPreference = "Stop";
trap { $host.SetShouldExit(1) }

go run github.com/onsi/ginkgo/v2/ginkgo -r --race -keep-going --randomize-suites
if ($LASTEXITCODE -ne 0) {
    throw "tests failed"
}
