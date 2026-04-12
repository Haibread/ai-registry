package domain

import (
	"encoding/json"
	"fmt"
	"time"
)

// A2AProtocolVersion is the A2A spec version this implementation targets.
// Ref: https://github.com/a2aproject/a2a (June 2025 shape)
const A2AProtocolVersion = "0.2.1"

// validAuthSchemes is the allowlist of authentication scheme types.
var validAuthSchemes = map[string]bool{
	"Bearer":        true,
	"ApiKey":        true,
	"OAuth2":        true,
	"OpenIdConnect": true,
}

// Agent is the top-level entity for an AI agent in the registry.
type Agent struct {
	ID          string
	PublisherID string
	Namespace   string // publisher slug
	Slug        string
	Name        string
	Description string
	Visibility  Visibility
	Status      ServerStatus
	Featured    bool
	Verified    bool
	Readme      string
	ViewCount   int
	CopyCount   int
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// AgentVersion is an immutable versioned release of an Agent.
// Once published_at is set, no fields may be mutated.
type AgentVersion struct {
	ID                 string
	AgentID            string
	Version            string // semver
	EndpointURL        string
	Skills             json.RawMessage // []AgentSkill
	Capabilities       json.RawMessage // AgentCapabilities
	Authentication     json.RawMessage // []AuthScheme
	DefaultInputModes  []string
	DefaultOutputModes []string
	Provider           json.RawMessage // AgentProvider
	DocumentationURL   string
	IconURL            string
	ProtocolVersion    string
	Status             VersionStatus // active | deprecated | deleted
	StatusMessage      string
	StatusChangedAt    time.Time
	PublishedAt        *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// IsPublished reports whether this version has been published.
func (v *AgentVersion) IsPublished() bool { return v.PublishedAt != nil }

// ── Validation types ─────────────────────────────────────────────────────────

// AgentSkill mirrors the A2A AgentSkill object.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// AuthScheme is one entry in the authentication array.
type AuthScheme struct {
	Scheme string `json:"scheme"` // Bearer | ApiKey | OAuth2 | OpenIdConnect
}

// ValidateSkills checks that the skills JSONB is a valid array and that each
// entry satisfies the structural requirements of the A2A AgentSkill schema.
func ValidateSkills(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("skills must not be empty")
	}
	var skills []AgentSkill
	if err := json.Unmarshal(raw, &skills); err != nil {
		return fmt.Errorf("skills must be a JSON array: %w", err)
	}
	if len(skills) == 0 {
		return fmt.Errorf("skills must contain at least one entry")
	}
	for i, s := range skills {
		if s.ID == "" {
			return fmt.Errorf("skills[%d].id is required", i)
		}
		if s.Name == "" {
			return fmt.Errorf("skills[%d].name is required", i)
		}
		if s.Description == "" {
			return fmt.Errorf("skills[%d].description is required", i)
		}
		if s.Tags == nil {
			return fmt.Errorf("skills[%d].tags is required (may be an empty array)", i)
		}
		for j, tag := range s.Tags {
			if tag == "" {
				return fmt.Errorf("skills[%d].tags[%d] must not be an empty string", i, j)
			}
		}
	}
	return nil
}

// ValidateAuthentication checks that each authentication entry uses an
// allowed scheme from the registry allowlist.
func ValidateAuthentication(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil // empty is allowed (public agent)
	}
	var schemes []AuthScheme
	if err := json.Unmarshal(raw, &schemes); err != nil {
		return fmt.Errorf("authentication must be a JSON array: %w", err)
	}
	for i, s := range schemes {
		if s.Scheme == "" {
			return fmt.Errorf("authentication[%d].scheme is required", i)
		}
		if !validAuthSchemes[s.Scheme] {
			return fmt.Errorf(
				"authentication[%d].scheme %q is not allowed (valid: Bearer, ApiKey, OAuth2, OpenIdConnect)",
				i, s.Scheme,
			)
		}
	}
	return nil
}
