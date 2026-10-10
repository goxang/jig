package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/goxang/jig/engine"
)

func nsCommand() *cobra.Command {
	var create, reset bool
	c := &cobra.Command{
		Use:   "ns [namespace]",
		Short: "Kubernetes namespaces: list them, switch this environment to one (kept until --reset), --create a new one",
		Example: `  jig -e k8s ns                 # namespaces, * the one in use
  jig -e k8s ns --create me-test # create it and switch to it
  jig -e k8s ns me-test          # switch
  jig -e k8s ns --reset          # back to jig.yaml's`,
		Args: cobra.MaximumNArgs(1),
		RunE: withApp(func(ctx context.Context, a *engine.App, args []string) error {
			switch {
			case reset:
				if err := a.UseNamespace(""); err != nil {
					return err
				}
				fmt.Println("namespace back to the one in " + a.Spec.File)
				return nil
			case len(args) == 0:
				names, err := a.Namespaces(ctx)
				if err != nil {
					return err
				}
				var rows [][]string
				for _, n := range names {
					mark := ""
					if n == a.Namespace() {
						mark = "*"
					}
					rows = append(rows, []string{mark, n})
				}
				printTable(os.Stdout, []string{"", "NAMESPACE"}, rows)
				return nil
			}
			ns := args[0]
			if create {
				if err := a.CreateNamespace(ctx, ns); err != nil {
					return err
				}
			}
			if err := a.UseNamespace(ns); err != nil {
				return err
			}
			fmt.Printf("%s now works in namespace %s (jig ns --reset undoes it)\n", a.Env.Name, ns)
			return nil
		}),
	}
	c.Flags().BoolVar(&create, "create", false, "create the namespace first")
	c.Flags().BoolVar(&reset, "reset", false, "go back to the namespace jig.yaml names")
	return c
}
