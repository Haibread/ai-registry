package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type problemDetail struct {
	Type     string         `json:"type"`
	Title    string         `json:"title"`
	Status   int            `json:"status"`
	Detail   string         `json:"detail,omitempty"`
	Instance string         `json:"instance,omitempty"`
	Errors   []fieldError   `json:"errors,omitempty"`
}

type fieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func writeProblem(w http.ResponseWriter, status int, slug, detail, instance string) {
	p := problemDetail{
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
