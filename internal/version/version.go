package version

var (
	// Name is the name of the binary.
	Name = "berglas"

	// Version is the main package version.
	Version = "source"

	// Commit is the git sha.
	Commit = "HEAD"

	// HumanVersion is the compiled version.
	HumanVersion = Name + " v" + Version + " (" + Commit + ")"
)
