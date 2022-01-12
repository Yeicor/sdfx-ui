//go:build freebsd || openbsd || netbsd || dragonfly || darwin || windows || linux || solaris
// +build freebsd openbsd netbsd dragonfly darwin windows linux solaris

package ui

import "github.com/fsnotify/fsnotify"

func newFsWatcher() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}
