// Package identity says who is calling: a person, the AI, an agent or an API token.
package identity

import (
	"os"
	"os/user"
)

const (
	KindUser  = "user"
	KindAI    = "ai"
	KindAgent = "agent"
	KindToken = "token"
)

type Subject struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Groups []string `json:"groups,omitempty"`
	Roles  []string `json:"roles,omitempty"`
}

func (s Subject) String() string { return s.Kind + ":" + s.Name }

// Local is the person at this terminal; locally they own everything.
func Local() Subject {
	name := os.Getenv("USER")
	if name == "" {
		if u, err := user.Current(); err == nil {
			name = u.Username
		}
	}
	return Subject{Kind: KindUser, Name: name, Roles: []string{"owner"}}
}

// AI is the assistant acting for on, with on's roles.
func AI(on Subject) Subject {
	return Subject{Kind: KindAI, Name: "ai@" + on.Name, Roles: on.Roles}
}
