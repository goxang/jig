// Package refs handles the URIs every resource and row can carry, such as svc://api or
// jira://PAY-12. Each scheme has one owner: core or a plugin.
package refs

import (
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Core owns these schemes.
var Core = []string{"svc", "env", "view", "job", "file"}

type Ref struct {
	Scheme string
	// Path is everything after scheme://, without query or fragment.
	Path     string
	Query    url.Values
	Fragment string
}

func Parse(s string) (Ref, error) {
	scheme, rest, ok := strings.Cut(s, "://")
	if !ok || scheme == "" || strings.ContainsAny(scheme, "/?#") {
		return Ref{}, fmt.Errorf("not a ref: %q", s)
	}
	r := Ref{Scheme: strings.ToLower(scheme)}
	rest, r.Fragment, _ = strings.Cut(rest, "#")
	rest, q, _ := strings.Cut(rest, "?")
	r.Path = rest
	if q != "" {
		v, err := url.ParseQuery(q)
		if err != nil {
			return Ref{}, fmt.Errorf("ref %q: %w", s, err)
		}
		r.Query = v
	}
	return r, nil
}

func (r Ref) String() string {
	s := r.Scheme + "://" + r.Path
	if len(r.Query) > 0 {
		s += "?" + r.Query.Encode()
	}
	if r.Fragment != "" {
		s += "#" + r.Fragment
	}
	return s
}

var hole = regexp.MustCompile(`\{([^{}]+)\}`)

// Expand fills a template such as svc://{service} from a row; a hole the row has no value for stays.
func Expand(tmpl string, row map[string]any) string {
	return hole.ReplaceAllStringFunc(tmpl, func(h string) string {
		v, ok := row[h[1:len(h)-1]]
		if !ok || v == nil {
			return h
		}
		return fmt.Sprint(v)
	})
}

// Registry maps each scheme to its owner.
type Registry struct {
	mu     sync.RWMutex
	owners map[string]string
}

func NewRegistry() *Registry {
	r := &Registry{owners: map[string]string{}}
	for _, s := range Core {
		r.owners[s] = "core"
	}
	return r
}

// Claim gives scheme to owner; a scheme already owned by someone else is an error.
func (r *Registry) Claim(scheme, owner string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	scheme = strings.ToLower(scheme)
	if have, ok := r.owners[scheme]; ok && have != owner {
		return fmt.Errorf("scheme %s:// is already owned by %s", scheme, have)
	}
	r.owners[scheme] = owner
	return nil
}

func (r *Registry) Owner(scheme string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	o, ok := r.owners[strings.ToLower(scheme)]
	return o, ok
}

func (r *Registry) Schemes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.owners))
	for s := range r.owners {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
