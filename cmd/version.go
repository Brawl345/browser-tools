package cmd

// Injected at build time via -ldflags. Defaults for local builds.
var (
	Commit = "dev"                    // git SHA of this build
	Repo   = "Brawl345/browser-tools" // owner/repo, fork-aware via CI
)
