package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON_StrictDecoding(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	tests := []struct {
		name     string
		body     string
		wantOK   bool
		wantCode int
	}{
		{
			name:   "valid single object",
			body:   `{"name":"alice"}`,
			wantOK: true,
		},
		{
			name:     "unknown field is rejected",
			body:     `{"name":"alice","is_admin":true}`,
			wantOK:   false,
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name:     "trailing data after object is rejected",
			body:     `{"name":"alice"}{"name":"bob"}`,
			wantOK:   false,
			wantCode: http.StatusUnprocessableEntity,
		},
		{
			name:     "malformed JSON is rejected",
			body:     `{"name":`,
			wantOK:   false,
			wantCode: http.StatusUnprocessableEntity,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/things", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()

			var v payload
			ok := decodeJSON(rec, req, &v)

			if ok != tc.wantOK {
				t.Fatalf("decodeJSON ok = %v, want %v (body=%q)", ok, tc.wantOK, tc.body)
			}
			if tc.wantOK {
				if v.Name != "alice" {
					t.Errorf("decoded name = %q, want alice", v.Name)
				}
				return
			}
			if rec.Code != tc.wantCode {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantCode)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", ct)
			}
		})
	}
}
