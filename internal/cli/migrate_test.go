package cli

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// TestMigrationTable checks every jig command docs/migration.md maps a rig command to exists.
func TestMigrationTable(t *testing.T) {
	doc, err := os.ReadFile("../../docs/migration.md")
	if err != nil {
		t.Fatal(err)
	}
	rows := regexp.MustCompile("(?m)^\\| `rig [^`]+` \\| `jig ([^`]+)` \\|$").FindAllStringSubmatch(string(doc), -1)
	if len(rows) < 60 {
		t.Fatalf("only %d rows", len(rows))
	}
	root := newRoot()
	for _, r := range rows {
		c, rest, err := root.Find(strings.Fields(r[1]))
		if err != nil || len(rest) > 0 || c.CommandPath() != "jig "+r[1] {
			t.Errorf("jig %s: no such command", r[1])
		}
	}
}
