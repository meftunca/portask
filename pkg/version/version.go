package version

const (
	// Version represents the current version of Portask
	Version = "1.0.0"

	// BuildDate will be set during build
	BuildDate = "2025-08-14"

	// GitCommit will be set during build
	GitCommit = ""

	// AppName is the application name
	AppName = "Portask"

	// AppDescription is the application description
	AppDescription = "Ultra High Performance Message Queue"
)

// GetVersionInfo returns formatted version information
func GetVersionInfo() map[string]string {
	return map[string]string{
		"name":        AppName,
		"version":     Version,
		"description": AppDescription,
		"build_date":  BuildDate,
		"git_commit":  GitCommit,
	}
}
