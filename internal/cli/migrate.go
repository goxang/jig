package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/goxang/jig/host"
	"github.com/goxang/jig/spec"
)

func migrateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: fmt.Sprintf("move a rig project to jig (rig.yaml to jig.yaml, .rig state, ~/.config/rig), or rewrite jig.yaml to version %d", spec.CurrentVersion),
		RunE: func(cmd *cobra.Command, _ []string) error {
			file := g.file
			if file == "" {
				var err error
				if file, err = spec.Find("."); err != nil {
					if _, rerr := os.Stat("rig.yaml"); rerr == nil {
						return migrateRig(cmd.Context(), ".")
					}
					return err
				}
			}
			from, changed, err := spec.Migrate(file)
			if err != nil {
				return err
			}
			if !changed {
				fmt.Printf("%s: already version %d, nothing to do\n", file, spec.CurrentVersion)
				return nil
			}
			fmt.Printf("%s %s: version %d → %d\n", green("✓"), file, from, spec.CurrentVersion)
			return nil
		},
	}
}

// migrateRig writes jig.yaml from rig.yaml and copies rig's state and config; rig's files stay.
func migrateRig(ctx context.Context, dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, "rig.yaml"))
	if err != nil {
		return err
	}
	out, notes := spec.FromRig(raw)
	out = append(out, []byte("\n# RIG_* variables for scripts written for rig; drop once they use JIG_*\ncompat: {rig_env: true}\n")...)
	file := filepath.Join(dir, "jig.yaml")
	if err := os.WriteFile(file, out, 0o644); err != nil {
		return err
	}
	fmt.Printf("%s jig.yaml from rig.yaml\n", green("✓"))
	for _, n := range notes {
		fmt.Println(amber("  ! ") + n)
	}
	// ai-audit and flow are rig's own; bin, run and logs are rebuilt
	skip := []string{"ai-audit.jsonl", "flow.json", "bin", "run", "logs"}
	if n, err := copyMissing(filepath.Join(dir, ".rig"), filepath.Join(dir, ".jig"), skip); err == nil && n > 0 {
		fmt.Printf("%s .rig → .jig (%d files)\n", green("✓"), n)
	}
	if ucd, err := os.UserConfigDir(); err == nil {
		if n, err := copyMissing(filepath.Join(ucd, "rig"), host.ConfigDir(), nil); err == nil && n > 0 {
			fmt.Printf("%s %s → %s (%d files)\n", green("✓"), filepath.Join(ucd, "rig"), host.ConfigDir(), n)
		}
	}
	p, _, err := spec.Load(file, "")
	if err != nil {
		return err
	}
	types := pluginTypes(p)
	if len(types) > 0 {
		if err := installer(dir).Add(ctx, types); err != nil {
			fmt.Println(amber("  ! ") + "plugins not pinned (" + err.Error() + "); later: jig plugin add " + strings.Join(types, " "))
		}
	}
	fmt.Println(dim("rig.yaml and .rig stay; delete them when jig does all you need"))
	return nil
}

// pluginTypes are the plugins the project's components, runtimes and imports name.
func pluginTypes(p *spec.Project) []string {
	var out []string
	add := func(cs map[string]*spec.Component) {
		for _, c := range cs {
			if c != nil && c.Type != "" {
				out = append(out, c.Type)
			}
		}
	}
	add(p.Components)
	for _, e := range p.Environments {
		if e.Runtime != nil && e.Runtime.Type != "" {
			out = append(out, e.Runtime.Type)
		}
		add(e.Components)
	}
	for _, im := range p.Imports {
		for k := range im {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// copyMissing copies src's files into dst, leaving files dst already has and names in skip.
func copyMissing(src, dst string, skip []string) (int, error) {
	n := 0
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if slices.Contains(skip, d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		to := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(to, 0o755)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		if _, err := os.Stat(to); !errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, _ := d.Info()
		n++
		return os.WriteFile(to, b, info.Mode().Perm())
	})
	return n, err
}

func lintCommand() *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "list jig.yaml values its imports, includes or plugins already give (--fix removes them)",
		RunE: func(*cobra.Command, []string) error {
			file := g.file
			if file == "" {
				var err error
				if file, err = spec.Find("."); err != nil {
					return err
				}
			}
			rs, fixed, err := spec.Redundant(file)
			if err != nil {
				return err
			}
			for _, r := range rs {
				fmt.Printf("%s:%d %s %s\n", filepath.Base(file), r.Line, r.Path, dim("same as "+r.From.String()))
			}
			if len(rs) == 0 {
				fmt.Println(green("✓") + " nothing redundant")
				return nil
			}
			if fix {
				if err := os.WriteFile(file, fixed, 0o644); err != nil {
					return err
				}
				fmt.Printf("%s removed %d\n", green("✓"), len(rs))
				return nil
			}
			return fmt.Errorf("%d redundant values (jig lint --fix removes them)", len(rs))
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "remove them from jig.yaml (comments stay; layout is re-indented)")
	cmd.Flags().Bool("redundant", true, "values the layers below jig.yaml already give (the only check so far)")
	return cmd
}
