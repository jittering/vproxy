//go:build linux || darwin

package main

func findConfigFile(path string, isDaemon bool) string {
	paths := []string{path}
	if !isDaemon {
		// look for dot file only for clients
		paths = append(paths, ".vproxy.conf")
	}

	paths = append(paths,
		homeConfPath(),
		"/usr/local/etc/vproxy.conf",
		"/opt/homebrew/etc/vproxy.conf",
		"/etc/vproxy.conf",
	)

	return findConfig(paths...)
}
