package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"radar.nvim/internal/protocol"
)

type Config struct {
	BaseURL  string
	Email    string
	APIToken string
}

type searchRequest struct {
	JQL        string   `json:"jql"`
	MaxResults int      `json:"maxResults"`
	Fields     []string `json:"fields"`
}

type searchResponse struct {
	Issues []issue `json:"issues"`
}

type issue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary   string `json:"summary"`
		IssueType *struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Status *struct {
			Name           string `json:"name"`
			StatusCategory *struct {
				Key  string `json:"key"`
				Name string `json:"name"`
			} `json:"statusCategory"`
		} `json:"status"`
	} `json:"fields"`
}

func FetchAssignedIssues(ctx context.Context, logger *slog.Logger) ([]protocol.Entity, error) {
	cfg, ok := configFromEnv()
	if !ok {
		logger.Debug("jira collector not configured")
		return nil, nil
	}

	issues, err := searchAssignedIssues(ctx, cfg)
	if err != nil {
		return nil, err
	}

	entities := make([]protocol.Entity, 0, len(issues))
	for _, issue := range issues {
		entities = append(entities, entityFromIssue(cfg, issue))
	}
	logger.Debug("collected jira issues", "count", len(entities))
	return entities, nil
}

func configFromEnv() (Config, bool) {
	cfg := Config{
		BaseURL:  strings.TrimRight(os.Getenv("RADAR_JIRA_BASE_URL"), "/"),
		Email:    os.Getenv("RADAR_JIRA_EMAIL"),
		APIToken: os.Getenv("RADAR_JIRA_API_TOKEN"),
	}
	return cfg, cfg.BaseURL != "" && cfg.Email != "" && cfg.APIToken != ""
}

func searchAssignedIssues(ctx context.Context, cfg Config) ([]issue, error) {
	request := searchRequest{
		JQL:        "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC",
		MaxResults: 100,
		Fields:     []string{"summary", "status", "issuetype", "priority"},
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	endpoint := cfg.BaseURL + "/rest/api/3/search"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(cfg.Email, cfg.APIToken)

	res, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("jira search failed: %s: %s", res.Status, strings.TrimSpace(string(data)))
	}

	var response searchResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Issues, nil
}

func entityFromIssue(cfg Config, issue issue) protocol.Entity {
	status := ""
	statusCategory := ""
	if issue.Fields.Status != nil {
		status = issue.Fields.Status.Name
		if issue.Fields.Status.StatusCategory != nil {
			statusCategory = issue.Fields.Status.StatusCategory.Key
		}
	}

	metadata := map[string]string{
		"key": issue.Key,
	}
	if statusCategory != "" {
		metadata["status_category"] = statusCategory
	}
	if issue.Fields.IssueType != nil {
		metadata["issue_type"] = issue.Fields.IssueType.Name
	}
	if issue.Fields.Priority != nil {
		metadata["priority"] = issue.Fields.Priority.Name
	}

	return protocol.Entity{
		ID:       "jira:issue:" + issue.Key,
		Source:   "jira",
		Kind:     "issue",
		Title:    issue.Key + " " + issue.Fields.Summary,
		URL:      jiraIssueURL(cfg.BaseURL, issue.Key),
		Status:   status,
		Metadata: metadata,
	}
}

func jiraIssueURL(baseURL string, key string) string {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + "/browse/" + key
	}
	parsed.Path = "/browse/" + key
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}
