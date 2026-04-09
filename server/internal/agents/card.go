// Package agents implements A2A Agent Card generation.
//
// Targets the A2A protocol specification as of June 2025:
// https://github.com/a2aproject/a2a (commit-stable shape at that date)
//
// AgentCard fields:
//   name, description, version, url, provider, iconUrl, documentationUrl,
//   capabilities (streaming, pushNotifications, stateTransitionHistory, extendedAgentCard),
//   defaultInputModes, defaultOutputModes,
//   skills[] (id, name, description, tags[], examples[], inputModes[], outputModes[]),
//   securitySchemes (Bearer | ApiKey | OAuth2 | OpenIdConnect)
package agents

import (
	"encoding/json"
	"fmt"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentCard is the A2A-compatible agent card document.
// Ref: https://github.com/a2aproject/a2a/blob/main/docs/specification.md §2.1
type AgentCard struct {
	Name               string             `json:"name"`
	Description        string             `json:"description,omitempty"`
	Version            string             `json:"version"`
	URL                string             `json:"url"`
	Provider           *AgentProvider     `json:"provider,omitempty"`
	IconURL            string             `json:"iconUrl,omitempty"`
	DocumentationURL   string             `json:"documentationUrl,omitempty"`
	Capabilities       AgentCapabilities  `json:"capabilities"`
	DefaultInputModes  []string           `json:"defaultInputModes"`
	DefaultOutputModes []string           `json:"defaultOutputModes"`
	Skills             []AgentSkill       `json:"skills"`
	SecuritySchemes    map[string]any     `json:"securitySchemes,omitempty"`
}

// AgentProvider describes the organisation that owns the agent.
type AgentProvider struct {
	Organization string `json:"organization"`
	URL          string `json:"url,omitempty"`
}

// AgentCapabilities lists the optional A2A features the agent supports.
type AgentCapabilities struct {
	Streaming               bool `json:"streaming,omitempty"`
	PushNotifications       bool `json:"pushNotifications,omitempty"`
	StateTransitionHistory  bool `json:"stateTransitionHistory,omitempty"`
	ExtendedAgentCard       bool `json:"extendedAgentCard,omitempty"`
}

// AgentSkill is one skill entry in the A2A AgentCard skills array.
type AgentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Examples    []string `json:"examples,omitempty"`
	InputModes  []string `json:"inputModes,omitempty"`
	OutputModes []string `json:"outputModes,omitempty"`
}

// GenerateCard produces an A2A-compatible AgentCard from an AgentRow and its
// latest published AgentVersion. Returns an error if the version's stored JSON
// fields cannot be decoded.
func GenerateCard(agent store.AgentRow, ver *domain.AgentVersion) (*AgentCard, error) {
	if ver == nil {
		return nil, fmt.Errorf("agent %s/%s has no published version", agent.Namespace, agent.Slug)
	}

	// Decode skills.
	var skills []AgentSkill
	if err := json.Unmarshal(ver.Skills, &skills); err != nil {
		return nil, fmt.Errorf("decoding skills: %w", err)
	}
	if skills == nil {
		skills = []AgentSkill{}
	}

	// Decode capabilities.
	var caps AgentCapabilities
	if len(ver.Capabilities) > 0 && string(ver.Capabilities) != "null" {
		if err := json.Unmarshal(ver.Capabilities, &caps); err != nil {
			return nil, fmt.Errorf("decoding capabilities: %w", err)
		}
	}

	// Decode provider.
	var provider *AgentProvider
	if len(ver.Provider) > 0 && string(ver.Provider) != "null" {
		if err := json.Unmarshal(ver.Provider, &provider); err != nil {
			return nil, fmt.Errorf("decoding provider: %w", err)
		}
	}

	// Build security schemes map from authentication array.
	securitySchemes, err := buildSecuritySchemes(ver.Authentication)
	if err != nil {
		return nil, fmt.Errorf("building security schemes: %w", err)
	}

	inputModes := ver.DefaultInputModes
	if len(inputModes) == 0 {
		inputModes = []string{"text/plain"}
	}
	outputModes := ver.DefaultOutputModes
	if len(outputModes) == 0 {
		outputModes = []string{"text/plain"}
	}

	card := &AgentCard{
		Name:               agent.Name,
		Description:        agent.Description,
		Version:            ver.Version,
		URL:                ver.EndpointURL,
		Provider:           provider,
		IconURL:            ver.IconURL,
		DocumentationURL:   ver.DocumentationURL,
		Capabilities:       caps,
		DefaultInputModes:  inputModes,
		DefaultOutputModes: outputModes,
		Skills:             skills,
		SecuritySchemes:    securitySchemes,
	}

	return card, nil
}

// buildSecuritySchemes converts the stored authentication JSON array into the
// A2A securitySchemes map shape.
//
// Input: [{"scheme":"Bearer"}, {"scheme":"OAuth2","tokenUrl":"https://..."}]
// Output: {"bearer": {"type":"http","scheme":"bearer"}, "oauth2": {...}}
func buildSecuritySchemes(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == "[]" {
		return nil, nil
	}

	var entries []map[string]any
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("decoding authentication: %w", err)
	}

	schemes := make(map[string]any, len(entries))
	for _, e := range entries {
		scheme, _ := e["scheme"].(string)
		switch scheme {
		case "Bearer":
			schemes["bearer"] = map[string]any{
				"type":   "http",
				"scheme": "bearer",
			}
		case "ApiKey":
			s := map[string]any{"type": "apiKey"}
			if in, ok := e["in"].(string); ok {
				s["in"] = in
			} else {
				s["in"] = "header"
			}
			if name, ok := e["name"].(string); ok {
				s["name"] = name
			} else {
				s["name"] = "X-API-Key"
			}
			schemes["apiKey"] = s
		case "OAuth2":
			s := map[string]any{"type": "oauth2"}
			if flows, ok := e["flows"]; ok {
				s["flows"] = flows
			}
			schemes["oauth2"] = s
		case "OpenIdConnect":
			s := map[string]any{"type": "openIdConnect"}
			if url, ok := e["openIdConnectUrl"].(string); ok {
				s["openIdConnectUrl"] = url
			}
			schemes["openIdConnect"] = s
		}
	}

	if len(schemes) == 0 {
		return nil, nil
	}
	return schemes, nil
}

// RegistryCard builds the global AgentCard for the registry itself, making it
// a first-class A2A citizen discoverable at /.well-known/agent-card.json.
func RegistryCard(baseURL string) *AgentCard {
	return &AgentCard{
		Name:        "AI Registry",
		Description: "Centralized registry for MCP servers and AI agents. Discover, publish, and manage AI ecosystem artifacts.",
		Version:     "0.1.0",
		URL:         baseURL + "/api/v1",
		Capabilities: AgentCapabilities{
			ExtendedAgentCard: false,
		},
		DefaultInputModes:  []string{"application/json"},
		DefaultOutputModes: []string{"application/json"},
		Skills: []AgentSkill{
			{
				ID:          "mcp-registry",
				Name:        "MCP Server Registry",
				Description: "Discover and manage Model Context Protocol servers.",
				Tags:        []string{"mcp", "registry", "discovery"},
			},
			{
				ID:          "agent-registry",
				Name:        "AI Agent Registry",
				Description: "Discover and manage AI agents with A2A-compatible agent cards.",
				Tags:        []string{"a2a", "agents", "registry", "discovery"},
			},
		},
	}
}
