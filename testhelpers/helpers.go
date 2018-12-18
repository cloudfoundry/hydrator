package helpers_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/runtime-spec/specs-go"
)

type Helpers struct {
	wincBin         string
	grootBin        string
	grootImageStore string
	diffBin         string
	hydrateBin      string
	debug           bool
	logFile         *os.File
}

func NewHelpers(wincBin, grootBin, grootImageStore, diffBin, hydrateBin string, debug bool) *Helpers {
	h := &Helpers{
		wincBin:         wincBin,
		grootBin:        grootBin,
		grootImageStore: grootImageStore,
		diffBin:         diffBin,
		debug:           debug,
		hydrateBin:      hydrateBin,
	}

	if h.debug {
		var err error
		h.logFile, err = ioutil.TempFile("", "log")
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
	}
	return h
}

func (h *Helpers) CreateHelloLayer(rootfsURI string) string {
	var err error

	containerId, bundlePath := h.CreateContainer(rootfsURI)
	h.StartContainer(containerId)

	var args = []string{"cmd.exe", "/c", "echo hi > C:\\hello.txt"}

	_, _, err = h.ExecInContainer(containerId, args, false)
	Expect(err).To(Succeed())

	h.DeleteContainer(containerId)

	layerDir, err := ioutil.TempDir("", "diffoutput")
	Expect(err).To(Succeed())

	newLayer := filepath.Join(layerDir, "hello-layer")

	_, _, err = h.Execute(exec.Command(h.diffBin, "-outputFile", newLayer, "-containerId", containerId, "-bundlePath", bundlePath))
	Expect(err).To(Succeed())

	err = os.RemoveAll(bundlePath)
	Expect(err).To(Succeed())

	h.DeleteVolume(containerId)

	return newLayer
}

func (h *Helpers) CopyOciImage(ociImagePath, newOciImagePath string) {
	newBlobsPath := filepath.Join(newOciImagePath, "blobs", "sha256")
	Expect(os.MkdirAll(newBlobsPath, 0755)).To(Succeed())

	copyFile(filepath.Join(ociImagePath, "index.json"), filepath.Join(newOciImagePath, "index.json"))
	copyFile(filepath.Join(ociImagePath, "oci-layout"), filepath.Join(newOciImagePath, "oci-layout"))

	blobsPath := filepath.Join(ociImagePath, "blobs", "sha256")
	files, err := ioutil.ReadDir(blobsPath)
	Expect(err).NotTo(HaveOccurred())
	for _, file := range files {
		blobName := file.Name()
		copyFile(filepath.Join(blobsPath, blobName), filepath.Join(newBlobsPath, blobName))
	}
}

func copyFile(src, dest string) {
	df, err := os.Create(dest)
	Expect(err).NotTo(HaveOccurred())
	defer df.Close()

	sf, err := os.Open(src)
	Expect(err).NotTo(HaveOccurred())
	defer sf.Close()

	_, err = io.Copy(df, sf)
	Expect(err).NotTo(HaveOccurred())
}

func (h *Helpers) RunHydrate(args []string) *gexec.Session {
	command := exec.Command(h.hydrateBin, args...)
	hydrateSess, err := gexec.Start(command, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return hydrateSess
}

func (h *Helpers) GenerateBundle(bundleSpec specs.Spec, bundlePath string) {
	ExpectWithOffset(1, os.MkdirAll(bundlePath, 0666)).To(Succeed())
	config, err := json.Marshal(&bundleSpec)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	configFile := filepath.Join(bundlePath, "config.json")
	ExpectWithOffset(1, ioutil.WriteFile(configFile, config, 0666)).To(Succeed())
}

func (h *Helpers) CreateContainer(rootfsURI string) (string, string) {
	bundlePath, err := ioutil.TempDir("", "winccontainer")
	Expect(err).ToNot(HaveOccurred())

	containerId := filepath.Base(bundlePath)
	bundleSpec := h.GenerateRuntimeSpec(h.CreateVolume(rootfsURI, containerId))

	h.GenerateBundle(bundleSpec, bundlePath)

	_, _, err = h.Execute(h.ExecCommand(h.wincBin, "create", "-b", bundlePath, containerId))
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return containerId, bundlePath
}

func (h *Helpers) StartContainer(containerId string) {
	Eventually(func() error {
		_, _, err := h.Execute(h.ExecCommand(h.wincBin, "start", containerId))
		return err
	}).Should(Succeed())
}

func (h *Helpers) DeleteContainer(id string) {
	if h.ContainerExists(id) {
		output, err := h.ExecCommand(h.wincBin, "delete", id).CombinedOutput()
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
	}
}

func (h *Helpers) CreateVolume(rootfsURI, containerId string) specs.Spec {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(h.grootBin, "--driver-store", h.grootImageStore, "create", rootfsURI, containerId)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), fmt.Sprintf("groot stdout: %s\n\n groot stderr: %s\n\n", stdOut.String(), stdErr.String()))
	var spec specs.Spec
	ExpectWithOffset(1, json.Unmarshal(stdOut.Bytes(), &spec)).To(Succeed())
	return spec
}

func (h *Helpers) GrootPull(rootfsURI string) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	cmd := exec.Command(h.grootBin, "--driver-store", h.grootImageStore, "pull", rootfsURI)
	cmd.Stdout = stdOut
	cmd.Stderr = stdErr
	ExpectWithOffset(1, cmd.Run()).To(Succeed(), fmt.Sprintf("groot stdout: %s\n\n groot stderr: %s\n\n", stdOut.String(), stdErr.String()))
}

func (h *Helpers) DeleteVolume(id string) {
	output, err := exec.Command(h.grootBin, "--driver-store", h.grootImageStore, "delete", id).CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), string(output))
}

func (h *Helpers) ExecInContainer(id string, args []string, detach bool) (*bytes.Buffer, *bytes.Buffer, error) {
	var defaultArgs []string

	defaultArgs = []string{"exec"}

	if detach {
		defaultArgs = append(defaultArgs, "-d")
	}

	defaultArgs = append(defaultArgs, id)

	return h.Execute(h.ExecCommand(h.wincBin, append(defaultArgs, args...)...))
}

func (h *Helpers) GenerateRuntimeSpec(baseSpec specs.Spec) specs.Spec {
	return specs.Spec{
		Version: specs.Version,
		Process: &specs.Process{
			Args: []string{"cmd.exe", "/C", "waitfor", "ever", "/t", "9999"},
			Cwd:  "C:\\",
		},
		Root: &specs.Root{
			Path: baseSpec.Root.Path,
		},
		Windows: &specs.Windows{
			LayerFolders: baseSpec.Windows.LayerFolders,
		},
	}
}

func (h *Helpers) Execute(c *exec.Cmd) (*bytes.Buffer, *bytes.Buffer, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	c.Stdout = io.MultiWriter(stdOut, GinkgoWriter)
	c.Stderr = io.MultiWriter(stdErr, GinkgoWriter)
	err := c.Run()

	return stdOut, stdErr, err
}

func (h *Helpers) ExecCommand(command string, args ...string) *exec.Cmd {
	allArgs := []string{}
	if h.debug {
		allArgs = append([]string{"--log", h.logFile.Name(), "--debug"}, args...)
	} else {
		allArgs = args[0:]
	}
	return exec.Command(command, allArgs...)
}
