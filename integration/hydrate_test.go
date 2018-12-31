package hydrate_test

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"strings"

	"code.cloudfoundry.org/archiver/extractor"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	specs "github.com/opencontainers/image-spec/specs-go"
	oci "github.com/opencontainers/image-spec/specs-go/v1"
)

var _ = Describe("Hydrate", func() {
	var hydrateArgs []string

	Describe("remove-layer", func() {
		BeforeEach(func() {
			// while it should be possible to run the remove-layer command,
			// on a Linux machine, our tests will not work on Linux because they create containers
			// We also do not have a need to run this command on Linux
			if runtime.GOOS != "windows" {
				Skip("skipping test on non-windows platforms")
			}
		})

		Context("when -ociImage is not provided", func() {
			It("should throw an error that says -ociImage is not provided", func() {
				hydrateArgs = []string{"remove-layer"}
				hydrateSess := helpers.RunHydrate(hydrateArgs)
				Eventually(hydrateSess).Should(gexec.Exit())
				Expect(hydrateSess.ExitCode()).ToNot(Equal(0))
				Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("ERROR: Missing option -ociImage"))
			})
		})

		Context("when -ociImage is provided", func() {
			var (
				testOciImagePath string
			)

			Context("the oci image is valid", func() {
				var (
					verificationBundlePath  string
					verificationContainerId string
					newLayer                string
					rootfsURI               string
				)

				BeforeEach(func() {
					var err error
					testOciImagePath, err = ioutil.TempDir("", "test-oci-image")
					Expect(err).NotTo(HaveOccurred())

					helpers.CopyOciImage(ociImagePath, testOciImagePath)

					// add a "hello" layer to be removed
					rootfsURI = fmt.Sprintf("oci:///%s", filepath.ToSlash(testOciImagePath))
					newLayer = helpers.CreateHelloLayer(rootfsURI)
					hydrateArgs := []string{"add-layer", "--layer", newLayer, "--ociImage", testOciImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)
					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).To(Equal(0))
				})

				AfterEach(func() {
					Expect(os.RemoveAll(testOciImagePath)).To(Succeed())
					Expect(os.RemoveAll(filepath.Dir(newLayer))).To(Succeed())
					helpers.DeleteContainer(verificationContainerId)
					helpers.DeleteVolume(verificationContainerId)
					Expect(os.RemoveAll(verificationBundlePath)).To(Succeed())
				})

				It("should remove the top layer from the oci image", func() {
					hydrateArgs := []string{"remove-layer", "--ociImage", testOciImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).To(Equal(0))

					verificationContainerId, verificationBundlePath = helpers.CreateContainer(rootfsURI)

					_, _, err := helpers.ExecInContainer(verificationContainerId, []string{"cmd.exe", "/c", "type C:\\hello.txt"}, false)
					Expect(err).NotTo(Succeed())

					im := loadManifest(testOciImagePath)
					Expect(im.Annotations).NotTo(HaveKey("hydrator.layerAdded"))
				})
			})

			Context("when ociImage does not exist", func() {
				It("exits with an error", func() {
					hydrateArgs := []string{"remove-layer", "--ociImage", "image/that/doesnt/exist"}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("image\\that\\doesnt\\exist\\index.json: The system cannot find the path specified"))
				})
			})

			Context("when ociImage is not valid", func() {
				var invalidImagePath string

				BeforeEach(func() {
					var err error
					invalidImagePath, err = ioutil.TempDir("", "invalid-image")
					Expect(err).NotTo(HaveOccurred())
				})
				AfterEach(func() {
					Expect(os.RemoveAll(invalidImagePath)).To(Succeed())
				})

				It("exits with an error", func() {
					hydrateArgs := []string{"remove-layer", "--ociImage", invalidImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("couldn't load index.json"))
				})
			})
		})
	})
	Describe("add-layer", func() {
		BeforeEach(func() {
			// while it should be possible to run the add-layer command,
			// on a Linux machine, our tests will not work on Linux because they create containers
			// We also do not have a need to run this command on Linux
			if runtime.GOOS != "windows" {
				Skip("skipping test on non-windows platforms")
			}
		})

		Context("when -layer is provided but not -ociImage", func() {
			It("should throw an error that says -ociImage not provided", func() {
				hydrateArgs = []string{"add-layer", "--layer", "some-layer"}
				hydrateSess := helpers.RunHydrate(hydrateArgs)
				Eventually(hydrateSess).Should(gexec.Exit())
				Expect(hydrateSess.ExitCode()).ToNot(Equal(0))
				Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("ERROR: Missing option -ociImage"))
			})
		})

		Context("when -ociImage is provided but not -layer", func() {
			It("should throw an error that says -layer not provided", func() {
				hydrateArgs = []string{"add-layer", "--ociImage", "some-oci-image"}
				hydrateSess := helpers.RunHydrate(hydrateArgs)
				Eventually(hydrateSess).Should(gexec.Exit())
				Expect(hydrateSess.ExitCode()).ToNot(Equal(0))
				Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("ERROR: Missing option -layer"))
			})
		})

		Context("when exactly -layer and -ociImage options are provided", func() {
			var (
				testOciImagePath string
				rootfsURI        string
				newLayer         string
			)

			BeforeEach(func() {
				var err error
				testOciImagePath, err = ioutil.TempDir("", "test-oci-image")
				Expect(err).NotTo(HaveOccurred())

				helpers.CopyOciImage(ociImagePath, testOciImagePath)
				rootfsURI = fmt.Sprintf("oci:///%s", filepath.ToSlash(testOciImagePath))

				newLayer = helpers.CreateHelloLayer(rootfsURI)
			})

			AfterEach(func() {
				Expect(os.RemoveAll(testOciImagePath)).To(Succeed())
				Expect(os.RemoveAll(filepath.Dir(newLayer))).To(Succeed())
			})

			Context("the layer and oci image are valid", func() {
				var (
					verificationBundlePath  string
					verificationContainerId string
				)

				AfterEach(func() {
					helpers.DeleteContainer(verificationContainerId)
					helpers.DeleteVolume(verificationContainerId)
					Expect(os.RemoveAll(verificationBundlePath)).To(Succeed())
				})

				It("should modify the oci image with the layer added", func() {
					hydrateArgs := []string{"add-layer", "--layer", newLayer, "--ociImage", testOciImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).To(Equal(0))

					verificationContainerId, verificationBundlePath = helpers.CreateContainer(rootfsURI)

					stdOut, _, err := helpers.ExecInContainer(verificationContainerId, []string{"cmd.exe", "/c", "type C:\\hello.txt"}, false)
					Expect(err).To(Succeed())

					Expect(stdOut.String()).To(ContainSubstring("hi"))

					im := loadManifest(testOciImagePath)
					Expect(im.Annotations).To(HaveKeyWithValue("hydrator.layerAdded", "true"))
				})
			})

			Context("when layer does not exist", func() {
				It("exits with an error", func() {
					hydrateArgs := []string{"add-layer", "--layer", "layer/that/doesnt/exist", "--ociImage", testOciImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("The system cannot find the path specified"))
				})
			})

			Context("when ociImage does not exist", func() {
				It("exits with an error", func() {
					hydrateArgs := []string{"add-layer", "--layer", newLayer, "--ociImage", "image/that/doesnt/exist"}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("image\\that\\doesnt\\exist\\blobs\\sha256 directory missing"))
				})
			})

			Context("when layer is not valid", func() {
				var invalidLayerPath string

				BeforeEach(func() {
					tempFile, err := ioutil.TempFile("", "invalid-layer")
					Expect(err).NotTo(HaveOccurred())
					defer tempFile.Close()

					invalidLayerPath = tempFile.Name()
					_, err = tempFile.Write([]byte("invalid layer"))
					Expect(err).NotTo(HaveOccurred())

				})

				AfterEach(func() {
					Expect(os.RemoveAll(invalidLayerPath)).To(Succeed())
				})

				It("exits with an error", func() {
					hydrateArgs := []string{"add-layer", "--layer", invalidLayerPath, "--ociImage", testOciImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("invalid layer"))
				})
			})

			Context("when ociImage is not valid", func() {
				var invalidImagePath string

				BeforeEach(func() {
					var err error
					invalidImagePath, err = ioutil.TempDir("", "invalid-image")
					Expect(err).NotTo(HaveOccurred())
				})
				AfterEach(func() {
					Expect(os.RemoveAll(invalidImagePath)).To(Succeed())
				})

				It("exits with an error", func() {
					hydrateArgs := []string{"add-layer", "--layer", newLayer, "--ociImage", invalidImagePath}
					hydrateSess := helpers.RunHydrate(hydrateArgs)

					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).NotTo(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("is not a valid OCI image"))
				})
			})
		})
	})

	Describe("download", func() {
		var (
			outputDir        string
			imageName        string
			imageTag         string
			imageTarballName string
			imageContentsDir string
		)

		BeforeEach(func() {
			var err error
			outputDir, err = ioutil.TempDir("", "hydrateOutput")
			Expect(err).NotTo(HaveOccurred())

			imageContentsDir, err = ioutil.TempDir("", "image-contents")
			Expect(err).NotTo(HaveOccurred())

			imageName = "pivotalgreenhouse/windows2016fs-hydrate"
			imageTag = "1.0.0"
			nameParts := strings.Split(imageName, "/")
			Expect(len(nameParts)).To(Equal(2))
			imageTarballName = fmt.Sprintf("%s-%s.tgz", nameParts[1], imageTag)

			hydrateArgs = []string{}
		})

		AfterEach(func() {
			Expect(os.RemoveAll(outputDir)).To(Succeed())
			Expect(os.RemoveAll(imageContentsDir)).To(Succeed())
		})

		Context("when provided an output directory", func() {
			Context("when provided an image tag", func() {
				BeforeEach(func() {
					hydrateArgs = []string{"download", "--outputDir", outputDir, "--image", imageName, "--tag", imageTag}
				})

				It("downloads all the layers with the required metadata files", func() {
					hydrateSess := helpers.RunHydrate(hydrateArgs)
					Eventually(hydrateSess).Should(gexec.Exit(0))

					tarball := filepath.Join(outputDir, imageTarballName)
					extractTarball(tarball, imageContentsDir)

					ociLayoutFile := filepath.Join(imageContentsDir, "oci-layout")

					var il oci.ImageLayout
					content, err := ioutil.ReadFile(ociLayoutFile)
					Expect(err).NotTo(HaveOccurred())
					Expect(json.Unmarshal(content, &il)).To(Succeed())
					Expect(il.Version).To(Equal(specs.Version))

					im := loadManifest(imageContentsDir)
					Expect(im.Annotations).NotTo(HaveKey("hydrator.layerAdded"))

					ic := loadConfig(imageContentsDir)

					for i, layer := range im.Layers {
						Expect(layer.MediaType).To(Equal(oci.MediaTypeImageLayerGzip))

						layerFile := filename(imageContentsDir, layer)
						fi, err := os.Stat(layerFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(fi.Size()).To(Equal(layer.Size))

						Expect(sha256Sum(layerFile)).To(Equal(layer.Digest.Encoded()))

						Expect(diffID(layerFile)).To(Equal(ic.RootFS.DiffIDs[i].Encoded()))
					}
				})

				Describe("tarball sha", func() {
					It("creates an identical tarball when run multiple times", func() {
						hydrateSess := helpers.RunHydrate([]string{"download", "--outputDir", outputDir, "--image", imageName, "--tag", imageTag})
						Eventually(hydrateSess).Should(gexec.Exit(0))
						tarballPath := filepath.Join(outputDir, imageTarballName)
						actualSha1 := sha256Sum(tarballPath)
						Expect(os.Remove(tarballPath)).To(Succeed())

						hydrateSess = helpers.RunHydrate([]string{"download", "--outputDir", outputDir, "--image", imageName, "--tag", imageTag})
						Eventually(hydrateSess).Should(gexec.Exit(0))
						actualSha2 := sha256Sum(filepath.Join(outputDir, imageTarballName))

						Expect(actualSha1).To(Equal(actualSha2))
					})
				})

				Context("when the --noTarball flag is specified", func() {
					BeforeEach(func() {
						hydrateArgs = []string{"download", "--outputDir", outputDir, "--image", imageName, "--tag", imageTag, "--noTarball"}
					})

					It("downloads the layers and metadata to the output dir without creating a tarball", func() {
						hydrateSess := helpers.RunHydrate(hydrateArgs)
						Eventually(hydrateSess).Should(gexec.Exit(0))

						ociLayoutFile := filepath.Join(outputDir, "oci-layout")

						var il oci.ImageLayout
						content, err := ioutil.ReadFile(ociLayoutFile)
						Expect(err).NotTo(HaveOccurred())
						Expect(json.Unmarshal(content, &il)).To(Succeed())
						Expect(il.Version).To(Equal(specs.Version))

						im := loadManifest(outputDir)
						Expect(im.Annotations).NotTo(HaveKey("hydrator.layerAdded"))

						ic := loadConfig(outputDir)

						for i, layer := range im.Layers {
							Expect(layer.MediaType).To(Equal(oci.MediaTypeImageLayerGzip))

							layerFile := filename(outputDir, layer)
							fi, err := os.Stat(layerFile)
							Expect(err).NotTo(HaveOccurred())
							Expect(fi.Size()).To(Equal(layer.Size))

							Expect(sha256Sum(layerFile)).To(Equal(layer.Digest.Encoded()))

							Expect(diffID(layerFile)).To(Equal(ic.RootFS.DiffIDs[i].Encoded()))
						}
					})
				})

				Context("when not provided an image tag", func() {
					BeforeEach(func() {
						imageTag = "latest"
						nameParts := strings.Split(imageName, "/")
						Expect(len(nameParts)).To(Equal(2))
						imageTarballName = fmt.Sprintf("%s-%s.tgz", nameParts[1], imageTag)
						hydrateArgs = []string{"download", "--outputDir", outputDir, "--image", imageName}
					})

					It("downloads the latest image version", func() {
						hydrateSess := helpers.RunHydrate(hydrateArgs)
						Eventually(hydrateSess).Should(gexec.Exit(0))

						tarball := filepath.Join(outputDir, imageTarballName)
						extractTarball(tarball, imageContentsDir)

						im := loadManifest(imageContentsDir)
						ic := loadConfig(imageContentsDir)

						for i, layer := range im.Layers {
							Expect(layer.MediaType).To(Equal(oci.MediaTypeImageLayerGzip))

							layerFile := filename(imageContentsDir, layer)
							fi, err := os.Stat(layerFile)
							Expect(err).NotTo(HaveOccurred())
							Expect(fi.Size()).To(Equal(layer.Size))

							Expect(sha256Sum(layerFile)).To(Equal(layer.Digest.Encoded()))

							Expect(layer.MediaType).To(Equal(oci.MediaTypeImageLayerGzip))
							Expect(diffID(layerFile)).To(Equal(ic.RootFS.DiffIDs[i].Encoded()))
						}
					})
				})
			})

			Context("when not provided an image", func() {
				BeforeEach(func() {
					hydrateArgs = []string{"download", "--outputDir", outputDir}
				})

				It("errors", func() {
					hydrateSess := helpers.RunHydrate(hydrateArgs)
					Eventually(hydrateSess).Should(gexec.Exit())
					Expect(hydrateSess.ExitCode()).ToNot(Equal(0))
					Expect(string(hydrateSess.Err.Contents())).To(ContainSubstring("ERROR: No image name provided"))
				})
			})
		})

		Context("when the output directory does not exist", func() {
			BeforeEach(func() {
				hydrateArgs = []string{"download", "--image", imageName, "--tag", imageTag, "--outputDir", filepath.Join(outputDir, "random-dir")}
			})

			It("creates it and outputs the image tarball to that directory", func() {
				hydrateSess := helpers.RunHydrate(hydrateArgs)
				Eventually(hydrateSess).Should(gexec.Exit(0))
				Expect(filepath.Join(outputDir, "random-dir", imageTarballName)).To(BeAnExistingFile())
			})
		})

		Context("when no output directory is provided", func() {
			BeforeEach(func() {
				hydrateArgs = []string{"download", "--image", imageName, "--tag", imageTag}
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), imageTarballName))).To(Succeed())
			})

			AfterEach(func() {
				Expect(os.RemoveAll(filepath.Join(os.TempDir(), imageTarballName))).To(Succeed())
			})

			It("outputs to the system temp directory", func() {
				hydrateSess := helpers.RunHydrate(hydrateArgs)
				Eventually(hydrateSess).Should(gexec.Exit(0))
				Expect(filepath.Join(os.TempDir(), imageTarballName)).To(BeAnExistingFile())
			})
		})
	})
})

func extractTarball(path string, outputDir string) {
	err := extractor.NewTgz().Extract(path, outputDir)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
}

func sha256Sum(file string) string {
	Expect(file).To(BeAnExistingFile())
	f, err := os.Open(file)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	h := sha256.New()
	_, err = io.Copy(h, f)
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%x", h.Sum(nil))
}

func diffID(file string) string {
	Expect(file).To(BeAnExistingFile())
	f, err := os.Open(file)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	gz, err := gzip.NewReader(f)
	Expect(err).NotTo(HaveOccurred())
	defer gz.Close()

	h := sha256.New()
	_, err = io.Copy(h, gz)
	Expect(err).NotTo(HaveOccurred())
	return fmt.Sprintf("%x", h.Sum(nil))
}

func filename(dir string, desc oci.Descriptor) string {
	return filepath.Join(dir, "blobs", desc.Digest.Algorithm().String(), desc.Digest.Encoded())
}

func loadIndex(outDir string) oci.Index {
	var ii oci.Index
	content, err := ioutil.ReadFile(filepath.Join(outDir, "index.json"))
	Expect(err).NotTo(HaveOccurred())
	Expect(json.Unmarshal(content, &ii)).To(Succeed())

	return ii
}

func loadManifest(outDir string) oci.Manifest {
	ii := loadIndex(outDir)

	content, err := ioutil.ReadFile(filename(outDir, ii.Manifests[0]))
	Expect(err).NotTo(HaveOccurred())

	var im oci.Manifest
	Expect(json.Unmarshal(content, &im)).To(Succeed())
	return im
}

func loadConfig(outDir string) oci.Image {
	im := loadManifest(outDir)

	content, err := ioutil.ReadFile(filename(outDir, im.Config))
	Expect(err).NotTo(HaveOccurred())

	var ic oci.Image
	Expect(json.Unmarshal(content, &ic)).To(Succeed())
	return ic
}
