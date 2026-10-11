// Package skills carries the agent skills jig installs into a project (jig skill --install).
package skills

import _ "embed"

//go:embed jig/SKILL.md
var Jig string

//go:embed jig-plugin/SKILL.md
var Plugin string
