// +build windows

package helpers_test

import (
	"github.com/Microsoft/hcsshim"
	. "github.com/onsi/gomega"
)

func (h *Helpers) ContainerExists(containerId string) bool {
	query := hcsshim.ComputeSystemQuery{
		IDs: []string{containerId},
	}
	containers, err := hcsshim.GetContainers(query)
	ExpectWithOffset(1, err).ToNot(HaveOccurred())
	return len(containers) > 0
}
