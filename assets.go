package boringstack

import "embed"

// Assets contains the canonical project scaffold and deployment templates.
// Keeping both in one embedded filesystem lets the CLI work offline after it
// is installed without maintaining a second copy of the deploy artifacts.
//
//go:embed all:scaffold/base all:templates
var Assets embed.FS
