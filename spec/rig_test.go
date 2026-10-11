package spec

import (
	"strings"
	"testing"
)

func TestFromRig(t *testing.T) {
	in := `# yaml-language-server: $schema=https://raw.githubusercontent.com/goxang/rig/main/spec/rig.schema.json
version: 1
# RIG_NAMESPACE=me rig -e k8s task bootstrap
tasks:
  ship: {steps: ["rig -e $ENV -y task build", "rig do db seed && rig up --build", "(rig test)"]}
  cfg: {steps: ["cat rig.yaml .rig/local/state.json", "prom --config scripts/rig/prometheus.yml"]}
  help: "rig would ship it, then rig, the tool"
vars: {NS: "${RIG_NAMESPACE:-switch}", CM: my-rig}
flow:
  links: [a]
  open: x
dashboards: {}
`
	want := `# yaml-language-server: $schema=https://raw.githubusercontent.com/goxang/jig/main/spec/jig.schema.json
version: 1
# RIG_NAMESPACE=me jig -e k8s task bootstrap
tasks:
  ship: {steps: ["jig -e $ENV -y task build", "jig do db seed && jig up --build", "(jig test)"]}
  cfg: {steps: ["cat jig.yaml .jig/local/state.json", "prom --config scripts/rig/prometheus.yml"]}
  help: "jig would ship it, then jig, the tool"
vars: {NS: "${RIG_NAMESPACE:-switch}", CM: my-rig}
dashboards: {}
`
	out, notes := FromRig([]byte(in))
	if string(out) != want {
		t.Errorf("got\n%s", out)
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "flow") {
		t.Errorf("notes %v", notes)
	}
}
