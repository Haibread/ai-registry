package auth_test

import (
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
)

func TestKeycloakClaims_IsAdmin(t *testing.T) {
	tests := []struct {
		name  string
		roles []string
		want  bool
	}{
		{name: "admin role present", roles: []string{"default-roles-registry", "admin"}, want: true},
		{name: "only admin", roles: []string{"admin"}, want: true},
		{name: "no admin role", roles: []string{"viewer", "editor"}, want: false},
		{name: "empty roles", roles: []string{}, want: false},
		{name: "nil roles", roles: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &auth.KeycloakClaims{
				RealmAccess: auth.RealmAccess{Roles: tt.roles},
			}
			if got := c.IsAdmin(); got != tt.want {
				t.Errorf("IsAdmin() = %v, want %v", got, tt.want)
			}
		})
	}
}
