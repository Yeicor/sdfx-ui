//go:build !linux
// +build !linux

package ui

import (
	"os"
)

func signals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
