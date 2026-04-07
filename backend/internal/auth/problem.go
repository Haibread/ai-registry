package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// problem is the RFC 7807 error body.
type problem struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func writeProblem(w http.ResponseWriter, status int, slug, detail, instance string) {
	p := problem{
		Type:     fmt.Sprintf("https://registry/errors/%s", slug),
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}
