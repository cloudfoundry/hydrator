// +build !windows

package helpers_test

func (h *Helpers) ContainerExists(containerId string) bool {
	panic("don't call ContainerExists on Linux")
}
