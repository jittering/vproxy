//go:build windows

package main

func findConfigFile(path string, isDaemon bool) string {
	paths := []string{path}
	if !isDaemon {
		// look for dot file only for clients
		paths = append(paths, ".vproxy.conf")
	}
	paths = append(paths, homeConfPath())
	return findConfig(paths...)
}
