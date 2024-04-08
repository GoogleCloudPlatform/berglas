package version

import (
	"runtime"
	"runtime/debug"
)

var (
	// Name is the name of the binary.
	name string
	Name = valueOrFallback(name, func() string { return "berglas" })

	// Version is the main package version.
	version string
	Version = valueOrFallback(version, func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			if v := info.Main.Version; v != "" {
				return v
			}
		}

		return "source"
	})

	// Commit is the git sha.
	commit string
	Commit = valueOrFallback(commit, func() string {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				if setting.Key == "vcs.revision" {
					return setting.Value
				}
			}
		}

		return "HEAD"
	})

	// OSArch is the operating system and architecture combination.
	OSArch = runtime.GOOS + "/" + runtime.GOARCH

	// HumanVersion is the compiled version.
	HumanVersion = Name + " " + Version + " (" + Commit + ", " + OSArch + ")"

	// UserAgent is the HTTP user agent string.
	userAgent string
	UserAgent = valueOrFallback(userAgent, func() string {
		return Name + "/" + Version + " (+https://github.com/GoogleCloudPlatform/berglas)"
	})
)

func valueOrFallback(val string, fn func() string) string {
	if val != "" {
		return val
	}
	return fn()
}
