package auth_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestDevRealm_Phase7Bindings locks the contract between the auth middleware
// and deploy/keycloak-realm-dev.json. Phase 7 silently 403'd in the dev
// realm for a release because the realm file shipped no groups, no
// group-membership mapper, and no non-admin user. This test fails fast if
// a future edit removes any of them.
//
// The Keycloak realm import schema is not pinned anywhere in this repo,
// so we hand-pick the fields we depend on rather than typing the full
// import.
func TestDevRealm_Phase7Bindings(t *testing.T) {
	path := filepath.Join("..", "..", "..", "deploy", "keycloak-realm-dev.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read realm file: %v", err)
	}

	var realm struct {
		Realm   string `json:"realm"`
		Roles   struct {
			Realm []struct {
				Name string `json:"name"`
			} `json:"realm"`
		} `json:"roles"`
		Groups []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"groups"`
		Clients []struct {
			ClientID        string `json:"clientId"`
			ProtocolMappers []struct {
				Name           string            `json:"name"`
				ProtocolMapper string            `json:"protocolMapper"`
				Config         map[string]string `json:"config"`
			} `json:"protocolMappers"`
		} `json:"clients"`
		Users []struct {
			Username   string   `json:"username"`
			RealmRoles []string `json:"realmRoles"`
			Groups     []string `json:"groups"`
		} `json:"users"`
	}
	if err := json.Unmarshal(raw, &realm); err != nil {
		t.Fatalf("parse realm file: %v", err)
	}

	if realm.Realm != "ai-registry" {
		t.Errorf("realm name = %q, want %q", realm.Realm, "ai-registry")
	}

	// Groups must match the workspace group_name bindings in
	// deploy/bootstrap.example.yaml plus the default
	// AUTH_REVIEWER_GROUP. Bumping a binding here means bumping the
	// bootstrap YAML in the same change.
	wantGroups := []string{
		"anthropic-core",
		"anthropic-labs",
		"openai-platform",
		"registry-reviewers",
	}
	gotGroups := make([]string, 0, len(realm.Groups))
	for _, g := range realm.Groups {
		gotGroups = append(gotGroups, g.Name)
	}
	for _, want := range wantGroups {
		if !slices.Contains(gotGroups, want) {
			t.Errorf("realm missing group %q (have %v)", want, gotGroups)
		}
	}

	// Both browser-facing clients must emit `groups` on the access
	// token. RequireWorkspaceWrite and RequireReviewer read this claim;
	// drop the mapper and every non-admin write returns 403.
	wantMapperOn := []string{"ai-registry-web", "ai-registry-cli"}
	for _, clientID := range wantMapperOn {
		assertGroupMapper(t, realm.Clients, clientID)
	}

	// Users that exercise each authorization path.
	wantUsers := map[string]struct {
		realmRoles []string
		groups     []string
	}{
		"admin@example.com": {
			realmRoles: []string{"admin"},
		},
		"author@example.com": {
			groups: []string{"/anthropic-core", "/anthropic-labs"},
		},
		"reviewer@example.com": {
			groups: []string{"/registry-reviewers"},
		},
		"user@example.com": {
			// No roles, no groups — the 403 baseline.
		},
	}
	for username, want := range wantUsers {
		assertUser(t, realm.Users, username, want.realmRoles, want.groups)
	}
}

// assertGroupMapper fails the test unless the named client carries an
// oidc-group-membership-mapper that emits the bare group name on the
// access token under the "groups" claim.
func assertGroupMapper(t *testing.T, clients []struct {
	ClientID        string `json:"clientId"`
	ProtocolMappers []struct {
		Name           string            `json:"name"`
		ProtocolMapper string            `json:"protocolMapper"`
		Config         map[string]string `json:"config"`
	} `json:"protocolMappers"`
}, clientID string) {
	t.Helper()
	for _, c := range clients {
		if c.ClientID != clientID {
			continue
		}
		for _, m := range c.ProtocolMappers {
			if m.ProtocolMapper != "oidc-group-membership-mapper" {
				continue
			}
			if got := m.Config["claim.name"]; got != "groups" {
				t.Errorf("%s: group mapper claim.name = %q, want %q", clientID, got, "groups")
			}
			if got := m.Config["full.path"]; got != "false" {
				t.Errorf("%s: group mapper full.path = %q, want %q (KeycloakClaims expects bare names)", clientID, got, "false")
			}
			if got := m.Config["access.token.claim"]; got != "true" {
				t.Errorf("%s: group mapper access.token.claim = %q, want %q", clientID, got, "true")
			}
			return
		}
		t.Errorf("%s: no oidc-group-membership-mapper configured", clientID)
		return
	}
	t.Errorf("client %q not found in realm", clientID)
}

func assertUser(t *testing.T, users []struct {
	Username   string   `json:"username"`
	RealmRoles []string `json:"realmRoles"`
	Groups     []string `json:"groups"`
}, username string, wantRoles, wantGroups []string) {
	t.Helper()
	for _, u := range users {
		if u.Username != username {
			continue
		}
		for _, role := range wantRoles {
			if !slices.Contains(u.RealmRoles, role) {
				t.Errorf("user %s missing realm role %q (have %v)", username, role, u.RealmRoles)
			}
		}
		for _, group := range wantGroups {
			if !slices.Contains(u.Groups, group) {
				t.Errorf("user %s missing group %q (have %v)", username, group, u.Groups)
			}
		}
		return
	}
	t.Errorf("user %q not found in realm", username)
}
