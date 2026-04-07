package agents_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/agents"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func makeAgentRow(ns, slug, name string) store.AgentRow {
	return store.AgentRow{
		Agent: domain.Agent{
			ID:          "01HZ000000000000000000000A",
			PublisherID: "01HZ000000000000000000000B",
			Namespace:   ns,
			Slug:        slug,
			Name:        name,
			Description: "Test agent description",
			Visibility:  domain.VisibilityPublic,
			Status:      domain.StatusPublished,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

func makeAgentVersion(version, endpoint string) *domain.AgentVersion {
	now := time.Now()
	return &domain.AgentVersion{
		ID:                 "01HZ000000000000000000000C",
		AgentID:            "01HZ000000000000000000000A",
		Version:            version,
		EndpointURL:        endpoint,
		Skills:             json.RawMessage(`[{"id":"s1","name":"Skill 1","description":"Does something","tags":["tag1"]}]`),
		Capabilities:       json.RawMessage(`{"streaming":true,"pushNotifications":false}`),
		Authentication:     json.RawMessage(`[{"scheme":"Bearer"}]`),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		Provider:           json.RawMessage(`{"organization":"Test Org","url":"https://example.com"}`),
		DocumentationURL:   "https://docs.example.com",
		IconURL:            "https://icon.example.com/icon.png",
		ProtocolVersion:    domain.A2AProtocolVersion,
		PublishedAt:        &now,
		ReleasedAt:         now,
	}
}

func TestGenerateCard_Basic(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}

	if card.Name != "My Agent" {
		t.Errorf("Name = %q, want %q", card.Name, "My Agent")
	}
	if card.Version != "1.0.0" {
		t.Errorf("Version = %q, want %q", card.Version, "1.0.0")
	}
	if card.URL != "https://agent.example.com/api" {
		t.Errorf("URL = %q, want %q", card.URL, "https://agent.example.com/api")
	}
	if card.DocumentationURL != "https://docs.example.com" {
		t.Errorf("DocumentationURL = %q", card.DocumentationURL)
	}
	if card.IconURL != "https://icon.example.com/icon.png" {
		t.Errorf("IconURL = %q", card.IconURL)
	}
}

func TestGenerateCard_Skills(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}

	if len(card.Skills) != 1 {
		t.Fatalf("Skills len = %d, want 1", len(card.Skills))
	}
	if card.Skills[0].ID != "s1" {
		t.Errorf("Skills[0].ID = %q, want %q", card.Skills[0].ID, "s1")
	}
}

func TestGenerateCard_EmptySkillsBecomesEmptyArray(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Skills = json.RawMessage(`[]`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if card.Skills == nil {
		t.Error("Skills should be an empty slice, not nil")
	}
	if len(card.Skills) != 0 {
		t.Errorf("Skills len = %d, want 0", len(card.Skills))
	}
}

func TestGenerateCard_Capabilities(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Capabilities = json.RawMessage(`{"streaming":true,"pushNotifications":true}`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if !card.Capabilities.Streaming {
		t.Error("Capabilities.Streaming should be true")
	}
	if !card.Capabilities.PushNotifications {
		t.Error("Capabilities.PushNotifications should be true")
	}
}

func TestGenerateCard_Provider(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if card.Provider == nil {
		t.Fatal("Provider should not be nil")
	}
	if card.Provider.Organization != "Test Org" {
		t.Errorf("Provider.Organization = %q, want %q", card.Provider.Organization, "Test Org")
	}
}

func TestGenerateCard_DefaultModesWhenEmpty(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.DefaultInputModes = nil
	ver.DefaultOutputModes = nil

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if len(card.DefaultInputModes) == 0 || card.DefaultInputModes[0] != "text/plain" {
		t.Errorf("DefaultInputModes = %v, want [text/plain]", card.DefaultInputModes)
	}
	if len(card.DefaultOutputModes) == 0 || card.DefaultOutputModes[0] != "text/plain" {
		t.Errorf("DefaultOutputModes = %v, want [text/plain]", card.DefaultOutputModes)
	}
}

func TestGenerateCard_SecuritySchemes(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Authentication = json.RawMessage(`[{"scheme":"Bearer"},{"scheme":"ApiKey","in":"header","name":"X-API-Key"}]`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if _, ok := card.SecuritySchemes["bearer"]; !ok {
		t.Error("SecuritySchemes should contain 'bearer'")
	}
	if _, ok := card.SecuritySchemes["apiKey"]; !ok {
		t.Error("SecuritySchemes should contain 'apiKey'")
	}
}

func TestGenerateCard_NoAuthentication(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Authentication = json.RawMessage(`[]`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	if card.SecuritySchemes != nil {
		t.Errorf("SecuritySchemes should be nil for empty auth, got %v", card.SecuritySchemes)
	}
}

func TestGenerateCard_NilVersion(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")

	_, err := agents.GenerateCard(agent, nil)
	if err == nil {
		t.Error("GenerateCard() with nil version should return error")
	}
}

func TestGenerateCard_InvalidSkillsJSON(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Skills = json.RawMessage(`{bad json}`)

	_, err := agents.GenerateCard(agent, ver)
	if err == nil {
		t.Error("GenerateCard() with invalid skills JSON should return error")
	}
}

func TestRegistryCard(t *testing.T) {
	card := agents.RegistryCard("https://registry.example.com")

	if card.Name != "AI Registry" {
		t.Errorf("Name = %q, want %q", card.Name, "AI Registry")
	}
	if card.URL != "https://registry.example.com/api/v1" {
		t.Errorf("URL = %q, want %q", card.URL, "https://registry.example.com/api/v1")
	}
	if len(card.Skills) < 2 {
		t.Errorf("Skills len = %d, want >= 2", len(card.Skills))
	}
	if len(card.DefaultInputModes) == 0 {
		t.Error("DefaultInputModes should not be empty")
	}
	if len(card.DefaultOutputModes) == 0 {
		t.Error("DefaultOutputModes should not be empty")
	}
}

func TestBuildSecuritySchemes_OAuth2(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Authentication = json.RawMessage(`[{"scheme":"OAuth2","flows":{"clientCredentials":{"tokenUrl":"https://auth.example.com/token","scopes":{}}}}]`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	s, ok := card.SecuritySchemes["oauth2"]
	if !ok {
		t.Fatal("SecuritySchemes should contain 'oauth2'")
	}
	m, ok := s.(map[string]any)
	if !ok {
		t.Fatalf("oauth2 scheme is not a map, got %T", s)
	}
	if m["type"] != "oauth2" {
		t.Errorf("oauth2 scheme type = %v, want oauth2", m["type"])
	}
}

func TestBuildSecuritySchemes_OpenIdConnect(t *testing.T) {
	agent := makeAgentRow("acme", "my-agent", "My Agent")
	ver := makeAgentVersion("1.0.0", "https://agent.example.com/api")
	ver.Authentication = json.RawMessage(`[{"scheme":"OpenIdConnect","openIdConnectUrl":"https://auth.example.com/.well-known/openid-configuration"}]`)

	card, err := agents.GenerateCard(agent, ver)
	if err != nil {
		t.Fatalf("GenerateCard() error = %v", err)
	}
	s, ok := card.SecuritySchemes["openIdConnect"]
	if !ok {
		t.Fatal("SecuritySchemes should contain 'openIdConnect'")
	}
	m := s.(map[string]any)
	if m["openIdConnectUrl"] != "https://auth.example.com/.well-known/openid-configuration" {
		t.Errorf("openIdConnectUrl = %v", m["openIdConnectUrl"])
	}
}
