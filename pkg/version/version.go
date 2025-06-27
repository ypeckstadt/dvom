package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the main version number that is being run at the moment.
	Version = "dev"

	// Commit is the git commit that was compiled. This will be filled in by the compiler.
	Commit = "unknown"

	// Date is the date the binary was built. This will be filled in by the compiler.
	Date = "unknown"

	// GoVersion is the version of Go that was used to compile the binary.
	GoVersion = runtime.Version()

	// Platform is the platform the binary was built for.
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// Info returns version information
func Info() string {
	return fmt.Sprintf("dvom version %s\nGit commit: %s\nBuild date: %s\nGo version: %s\nPlatform: %s",
		Version, Commit, Date, GoVersion, Platform)
}
