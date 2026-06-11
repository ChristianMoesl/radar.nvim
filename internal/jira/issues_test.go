package jira

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchAssignedIssuesUsesSearchJQLEndpoint(t *testing.T) {
	var called []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)
		if got := r.Header.Get("Authorization"); got != basicAuth("me@example.com", "token") {
			t.Fatalf("authorization = %q, want basic auth", got)
		}
		if r.Method != http.MethodPost || r.URL.Path != "/search/jql" {
			t.Fatalf("request = %s %s, want POST /search/jql", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchResponse{
			Issues: []issue{{Key: "ABC-123"}},
		})
	}))
	defer server.Close()

	issues, err := searchAssignedIssues(context.Background(), Config{
		APIBaseURL: server.URL,
		BaseURL:    "https://jira.example.com",
		Email:      "me@example.com",
		APIToken:   "token",
	})
	if err != nil {
		t.Fatalf("searchAssignedIssues() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Key != "ABC-123" {
		t.Fatalf("issues = %+v, want one ABC-123 issue", issues)
	}
	if len(called) != 1 {
		t.Fatalf("called %d endpoints, want 1", len(called))
	}
}

func TestSearchAssignedIssuesFallsBackToSearch(t *testing.T) {
	var called []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = append(called, r.Method+" "+r.URL.Path)
		switch r.Method + " " + r.URL.Path {
		case "POST /search/jql":
			http.Error(w, `{"errorMessages":["not supported"]}`, http.StatusNotFound)
		case "GET /search/jql":
			http.Error(w, `{"errorMessages":["not supported"]}`, http.StatusNotFound)
		case "POST /search":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(searchResponse{
				Issues: []issue{{Key: "ABC-456"}},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	issues, err := searchAssignedIssues(context.Background(), Config{
		APIBaseURL: server.URL,
		BaseURL:    "https://jira.example.com",
		Email:      "me@example.com",
		APIToken:   "token",
	})
	if err != nil {
		t.Fatalf("searchAssignedIssues() error = %v", err)
	}
	if len(issues) != 1 || issues[0].Key != "ABC-456" {
		t.Fatalf("issues = %+v, want one ABC-456 issue", issues)
	}
	want := []string{"POST /search/jql", "GET /search/jql", "POST /search"}
	if len(called) != len(want) {
		t.Fatalf("called %d endpoints, want %d", len(called), len(want))
	}
	for i := range want {
		if called[i] != want[i] {
			t.Fatalf("called[%d] = %q, want %q", i, called[i], want[i])
		}
	}
}

func basicAuth(user string, password string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+password))
}
