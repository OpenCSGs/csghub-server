package version

var (
	// GitRevision is the commit hash of the repo.
	// Injected during build.
	GitRevision = "0000000"

	//StarhubAPIVersion is the version of StarhubAPI.
	// Injected during build.
	StarhubAPIVersion = "v1.0"
)
