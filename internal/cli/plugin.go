package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/goxang/jig/contracts"
	"github.com/goxang/jig/host"
	"github.com/goxang/jig/spec"
)

// projectDir is where the project file is, or here when there is none.
func projectDir() string {
	if g.file != "" {
		abs, _ := filepath.Abs(g.file)
		return filepath.Dir(abs)
	}
	if f, err := spec.Find("."); err == nil {
		return filepath.Dir(f)
	}
	wd, _ := os.Getwd()
	return wd
}

func installer(dir string) *host.Installer {
	in := &host.Installer{Dir: dir, Registry: host.UserRegistry(), Yes: g.yes || os.Getenv("JIG_TRUST") == "project", Out: os.Stderr}
	if !in.Yes && term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd())) {
		in.Ask = askYes
	}
	return in
}

// ensurePlugins installs what jig.lock pins and this machine lacks; true means the project must reload.
func ensurePlugins(dir string) (bool, error) {
	changed, err := installer(dir).Ensure(context.Background())
	if err != nil && !errors.Is(err, host.ErrUntrusted) {
		fmt.Fprintln(os.Stderr, amber("plugins: ")+err.Error())
		return changed, nil
	}
	return changed, err
}

func pluginCommand() *cobra.Command {
	pc := &cobra.Command{Use: "plugin", Aliases: []string{"plugins"}, Short: "list, find, add, upgrade and switch plugins"}
	var major, project, follow bool

	pc.AddCommand(&cobra.Command{
		Use: "ls", Short: "the project's plugins: version, kind, where from, on or off",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := projectDir()
			cfg := pluginConfig(dir)
			all := map[string]*spec.Plugin{}
			for n, c := range cfg {
				cp := *c
				cp.Off = false
				all[n] = &cp
			}
			ms, warn := host.Discover(dir, all)
			lock, _ := host.ReadLock(dir)
			var rows [][]string
			for _, m := range ms {
				state, from := green("on"), dim(m.Dir)
				if c := cfg[m.Name]; c != nil && c.Off {
					state = dim("off")
				}
				if l := lock.Plugins[m.Name]; l != nil {
					from = l.Source
					if l.Pack != "" {
						from += dim(" (" + l.Pack + ")")
					}
				}
				rows = append(rows, []string{m.Name, m.Version, m.Kind(), state, from})
			}
			printTable(os.Stdout, []string{"PLUGIN", "VERSION", "KIND", "STATE", "FROM"}, rows)
			for _, w := range warn {
				fmt.Fprintln(os.Stderr, amber("! ")+w)
			}
			return nil
		},
	})
	pc.AddCommand(&cobra.Command{
		Use: "search [text]", Short: "plugins in the registry index",
		RunE: func(cmd *cobra.Command, args []string) error {
			ix, err := host.UserRegistry().Index(cmd.Context(), true)
			if err != nil {
				return err
			}
			q := strings.ToLower(strings.Join(args, " "))
			var rows [][]string
			for n, e := range ix.Plugins {
				if q == "" || strings.Contains(strings.ToLower(n+" "+e.Description), q) {
					rows = append(rows, []string{strings.Replace(n, "pack-", "pack/", 1), e.Latest(), e.Publisher, e.Description})
				}
			}
			sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
			printTable(os.Stdout, []string{"PLUGIN", "LATEST", "PUBLISHER", "DESCRIPTION"}, rows)
			return nil
		},
	})
	pc.AddCommand(&cobra.Command{
		Use: "check <name>", Short: "check a plugin against the contracts it implements, live", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return pluginCheck(cmd.Context(), os.Stdout, projectDir(), args[0])
		},
	})
	pc.AddCommand(&cobra.Command{
		Use: "info <name>", Short: "what a plugin provides and the permissions it has", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := projectDir()
			ms, _ := host.Discover(dir, pluginConfig(dir))
			for _, m := range ms {
				if m.Name == args[0] {
					return pluginInfo(cmd.Context(), os.Stdout, dir, m)
				}
			}
			ix, err := host.UserRegistry().Index(cmd.Context(), false)
			if err != nil {
				return err
			}
			e := ix.Plugins[args[0]]
			if e == nil {
				return fmt.Errorf("no plugin %s here or in the index", args[0])
			}
			v := e.Latest()
			fmt.Printf("%s %s %s\n%s\n", bold(args[0]), v, dim("(not installed: jig plugin add "+args[0]+")"), e.Description)
			if vv := e.Versions[v]; vv != nil {
				fmt.Printf("permissions: %s\n", vv.Permissions.String())
			}
			return nil
		},
	})
	logs := func(use, short string, file func(string) string) *cobra.Command {
		c := &cobra.Command{
			Use: use + " <name>", Short: short, Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return tailFile(cmd.Context(), file(args[0]), follow)
			},
		}
		c.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing as it grows")
		return c
	}
	pc.AddCommand(logs("logs", "the plugin's stderr and host/log lines", host.LogFile))
	pc.AddCommand(logs("trace", "the plugin's JSON-RPC exchange, recorded while JIG_TRACE names it (or is *)", host.TraceFile))
	pc.AddCommand(&cobra.Command{
		Use: "add <source>...", Short: "install plugins and pin them in jig.lock: a name, pack/<name>, host/owner/repo[/sub]@ref, a .tar.gz URL or a path",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer(projectDir()).Add(cmd.Context(), args)
		},
	})
	pc.AddCommand(&cobra.Command{
		Use: "rm <name>...", Aliases: []string{"remove"}, Short: "unpin plugins (and what a removed pack brought)", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer(projectDir()).Remove(args)
		},
	})
	up := &cobra.Command{
		Use: "up [name...]", Short: "upgrade plugins within their major version (all when none is named)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return installer(projectDir()).Up(cmd.Context(), args, major)
		},
	}
	up.Flags().BoolVar(&major, "major", false, "allow a new major version")
	pc.AddCommand(up)
	for _, on := range []bool{true, false} {
		verb := map[bool]string{true: "on", false: "off"}[on]
		c := &cobra.Command{
			Use: verb + " <name>...", Short: "turn plugins " + verb + " for you (.jig/local.yaml), or for everyone with --project",
			Args: cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				dir := projectDir()
				file := filepath.Join(dir, ".jig", "local.yaml")
				if project {
					f, err := spec.Find(dir)
					if err != nil {
						return err
					}
					file = f
				}
				for _, n := range args {
					if err := setYAML(file, []string{"plugins", n, "off"}, !on); err != nil {
						return err
					}
					fmt.Printf("  %s %s %s\n", green("✓"), n, verb)
				}
				return nil
			},
		}
		c.Flags().BoolVar(&project, "project", false, "write jig.yaml instead of .jig/local.yaml")
		pc.AddCommand(c)
	}
	pc.AddCommand(&cobra.Command{
		Use: "gc", Short: "delete installed plugin versions no project locks",
		RunE: func(cmd *cobra.Command, _ []string) error {
			removed, err := host.GC()
			for _, r := range removed {
				fmt.Println("  - " + r)
			}
			return err
		},
	})
	pc.AddCommand(&cobra.Command{
		Use: "vendor", Short: "copy the locked plugins into .jig/vendor for offline installs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			files, err := installer(projectDir()).Vendor()
			for _, f := range files {
				fmt.Println("  " + f)
			}
			return err
		},
	})
	return pc
}

func trustCommand() *cobra.Command {
	return &cobra.Command{
		Use: "trust", Short: "let this project's plugins run and install (workspace trust)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir := projectDir()
			if err := host.Trust(dir); err != nil {
				return err
			}
			fmt.Printf("  %s trusted %s\n", green("✓"), dir)
			in := installer(dir)
			_, err := in.Ensure(cmd.Context())
			return err
		},
	}
}

// pluginConfig is plugins: from the project as loaded (jig.yaml under .jig/local.yaml), without opening it.
func pluginConfig(dir string) map[string]*spec.Plugin {
	f, err := spec.Find(dir)
	if err != nil {
		return nil
	}
	p, _, err := spec.Load(f, g.env)
	if err != nil {
		return nil
	}
	return p.Plugins
}

func pluginCheck(ctx context.Context, w io.Writer, dir, name string) error {
	h := host.New(host.Options{Dir: dir}, pluginConfig(dir))
	defer h.Close()
	p := h.Get(name)
	if p == nil {
		return fmt.Errorf("no plugin %s here", name)
	}
	if len(p.Implements) == 0 {
		return fmt.Errorf("%s implements no contract", name)
	}
	pr, bad := p.Provides(ctx), 0
	for _, id := range p.Implements {
		c := contracts.Get(id)
		if c == nil {
			fmt.Fprintf(w, "%s %s\n", red("✗"), id+": no such contract")
			bad++
			continue
		}
		ps := c.Check(pr)
		if len(ps) == 0 {
			ps = c.CheckFrames(ctx, h, name)
		}
		if len(ps) == 0 {
			fmt.Fprintf(w, "%s %s\n", green("✓"), id)
		}
		for _, pb := range ps {
			fmt.Fprintf(w, "%s %s %s\n", red("✗"), id, pb)
		}
		bad += len(ps)
	}
	if bad > 0 {
		return fmt.Errorf("%d problems", bad)
	}
	return nil
}

func pluginInfo(ctx context.Context, w io.Writer, dir string, m *host.Manifest) error {
	fmt.Fprintf(w, "%s %s  %s\n", bold(m.Name), m.Version, dim(m.Kind()+" · "+m.Dir))
	if m.Description != "" {
		fmt.Fprintln(w, m.Description)
	}
	if m.Publisher != "" {
		fmt.Fprintln(w, "publisher:   "+m.Publisher)
	}
	fmt.Fprintln(w, "permissions: "+m.Permissions.String())
	if !host.Sandboxed() {
		fmt.Fprintln(w, dim("             advisory on this OS: no sandbox, calls are audited"))
	}
	h := host.New(host.Options{Dir: dir}, map[string]*spec.Plugin{})
	defer h.Close()
	if p := h.Get(m.Name); p != nil {
		pr := p.Provides(ctx)
		var rows [][]string
		for n, s := range pr.Sources {
			rows = append(rows, []string{"source", m.Name + "." + n, "read", s.Description})
		}
		for n, a := range pr.Actions {
			rows = append(rows, []string{"action", m.Name + "." + n, a.Effect, a.Description})
		}
		for n, im := range pr.Importers {
			rows = append(rows, []string{"importer", m.Name + "." + n, "", im.Description})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i][0]+rows[i][1] < rows[j][0]+rows[j][1] })
		if len(rows) > 0 {
			fmt.Fprintln(w)
			printTable(w, []string{"KIND", "NAME", "EFFECT", "HELP"}, rows)
		}
	}
	return nil
}

func tailFile(ctx context.Context, file string, follow bool) error {
	f, err := os.Open(file)
	if os.IsNotExist(err) {
		return fmt.Errorf("nothing at %s yet", file)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	if st, err := f.Stat(); err == nil && st.Size() > 64<<10 {
		_, _ = f.Seek(-64<<10, io.SeekEnd)
	}
	for {
		if _, err := io.Copy(os.Stdout, f); err != nil {
			return err
		}
		if !follow {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(300 * time.Millisecond):
		}
	}
}

// setYAML sets one nested key in a YAML file, keeping its comments and order.
func setYAML(file string, path []string, value any) error {
	var doc yaml.Node
	raw, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if len(raw) > 0 {
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	}
	n := doc.Content[0]
	for i, k := range path {
		if n.Kind != yaml.MappingNode {
			return fmt.Errorf("%s: %s is not a mapping", file, strings.Join(path[:i], "."))
		}
		var next *yaml.Node
		for j := 0; j+1 < len(n.Content); j += 2 {
			if n.Content[j].Value == k {
				next = n.Content[j+1]
			}
		}
		if i == len(path)-1 {
			var v yaml.Node
			if err := v.Encode(value); err != nil {
				return err
			}
			if next != nil {
				*next = v
			} else {
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, &v)
			}
			break
		}
		if next == nil || next.Kind == yaml.ScalarNode && next.Tag == "!!null" {
			if next == nil {
				next = &yaml.Node{}
				n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: k}, next)
			}
			*next = yaml.Node{Kind: yaml.MappingNode}
		}
		n = next
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	return os.WriteFile(file, out, 0o644)
}
