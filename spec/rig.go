package spec

import (
	"bytes"
	"regexp"
)

var (
	// rig as a word in a command or prose, not in a path (scripts/rig/), a variable (RIG_X) or a name (my-rig)
	rigWord  = regexp.MustCompile(`(^|[^\w./$-])rig(\s|\.yaml|[,;:)'"` + "`" + `]|$)`)
	rigDir   = regexp.MustCompile(`(^|[^\w])\.rig/`)
	topLevel = regexp.MustCompile(`^[A-Za-z_]`)
)

// FromRig is a rig.yaml as a jig.yaml: rig commands and paths become jig ones, and flow:, which jig
// has no counterpart for, is dropped. Notes say what changed beyond the renames.
func FromRig(src []byte) (out []byte, notes []string) {
	var buf bytes.Buffer
	skipping := false
	for _, line := range bytes.SplitAfter(src, []byte("\n")) {
		if topLevel.Match(line) {
			skipping = bytes.HasPrefix(line, []byte("flow:"))
			if skipping {
				notes = append(notes, "flow: dropped; jig has no flow screen")
			}
		}
		if skipping {
			continue
		}
		line = rigWord.ReplaceAll(line, []byte("${1}jig$2"))
		line = rigDir.ReplaceAll(line, []byte("$1.jig/"))
		line = bytes.ReplaceAll(line, []byte("goxang/rig/main/spec/rig.schema.json"), []byte("goxang/jig/main/spec/jig.schema.json"))
		buf.Write(line)
	}
	return buf.Bytes(), notes
}
