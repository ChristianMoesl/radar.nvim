package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"radar.nvim/internal/protocol"
)

type notification struct {
	ID         string `json:"id"`
	Reason     string `json:"reason"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Subject struct {
		Title string `json:"title"`
		Type  string `json:"type"`
		URL   string `json:"url"`
	} `json:"subject"`
}

type user struct {
	Login string `json:"login"`
}

type pullRequest struct {
	HTMLURL            string `json:"html_url"`
	State              string `json:"state"`
	Merged             bool   `json:"merged"`
	ClosedAt           string `json:"closed_at"`
	RequestedReviewers []user `json:"requested_reviewers"`
}

type searchPullRequest struct {
	Number     int    `json:"number"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	State      string `json:"state"`
	Draft      bool   `json:"isDraft"`
	ClosedAt   string `json:"closedAt"`
	Repository struct {
		FullName      string `json:"fullName"`
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
}

func FetchReviewRequests(ctx context.Context, logger *slog.Logger) ([]protocol.Item, error) {
	prs, err := fetchReviewRequestedPullRequests(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch review requested pull requests: %w", err)
	}
	logger.Debug("fetched github review requested pull requests", "count", len(prs))

	items := make([]protocol.Item, 0, len(prs))
	for _, pr := range prs {
		repo := pr.Repository.FullName
		if repo == "" {
			repo = pr.Repository.NameWithOwner
		}

		item := protocol.Item{
			ID:        fmt.Sprintf("github:review_request:%s:%d", repo, pr.Number),
			Kind:      "github_review_request",
			Title:     pr.Title,
			Repo:      repo,
			URL:       pr.URL,
			Attention: "attention",
			Reason:    "review requested",
		}
		item.Entities = []protocol.Entity{githubEntity(item, "pull_request")}
		items = append(items, item)
	}

	logger.Debug("github review requests classified", "count", len(items))
	return items, nil
}

func FetchAuthoredPullRequests(ctx context.Context, logger *slog.Logger) ([]protocol.Item, error) {
	prs, err := fetchAuthoredPullRequests(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch authored pull requests: %w", err)
	}
	logger.Debug("fetched authored github pull requests", "count", len(prs))

	items := make([]protocol.Item, 0, len(prs))
	for _, pr := range prs {
		repo := pr.Repository.FullName
		if repo == "" {
			repo = pr.Repository.NameWithOwner
		}

		reason := "open PR"
		if pr.Draft {
			reason = "draft PR"
		}

		item := protocol.Item{
			ID:        fmt.Sprintf("github:own_pr:%s:%d", repo, pr.Number),
			Kind:      "github_own_pr",
			Title:     pr.Title,
			Repo:      repo,
			URL:       pr.URL,
			Attention: "in_progress",
			Reason:    reason,
		}
		item.Entities = []protocol.Entity{githubEntity(item, "pull_request")}
		items = append(items, item)
	}

	logger.Debug("authored github pull requests classified", "count", len(items))
	return items, nil
}

func ResolveDonePullRequests(ctx context.Context, previous []protocol.Item, active []protocol.Item, logger *slog.Logger) []protocol.Item {
	if !EnsureCoreBudget(ctx, logger) {
		return keepTodaysDoneItems(previous)
	}

	activeIDs := make(map[string]bool, len(active))
	for _, item := range active {
		activeIDs[item.ID] = true
	}

	items := make([]protocol.Item, 0)
	for _, item := range previous {
		if item.Attention == "done" {
			if isToday(item.DoneAt) {
				items = append(items, item)
			}
			continue
		}

		if item.Kind != "github_own_pr" || activeIDs[item.ID] {
			continue
		}

		repo, number, ok := parseOwnPullRequestID(item.ID)
		if !ok {
			logger.Warn("could not parse github own PR id", "id", item.ID)
			continue
		}

		pr, err := fetchPullRequest(ctx, fmt.Sprintf("/repos/%s/pulls/%d", repo, number))
		if err != nil {
			logger.Warn("could not resolve previous github PR", "id", item.ID, "error", err)
			continue
		}
		if pr.State != "closed" || !isToday(pr.ClosedAt) {
			continue
		}

		reason := "closed today"
		if pr.Merged {
			reason = "merged today"
		}
		done := protocol.Item{
			ID:        fmt.Sprintf("github:done_pr:%s:%d", repo, number),
			Kind:      "github_done_pr",
			Title:     item.Title,
			Repo:      item.Repo,
			URL:       item.URL,
			Attention: "done",
			Reason:    reason,
			DoneAt:    pr.ClosedAt,
			Entities:  item.Entities,
		}
		done.Entities = append(done.Entities, githubEntity(done, "pull_request"))
		items = append(items, done)
	}

	logger.Debug("resolved done github pull requests", "count", len(items))
	return items
}

func keepTodaysDoneItems(previous []protocol.Item) []protocol.Item {
	items := make([]protocol.Item, 0)
	for _, item := range previous {
		if item.Attention == "done" && isToday(item.DoneAt) {
			items = append(items, item)
		}
	}
	return items
}

func githubEntity(item protocol.Item, kind string) protocol.Entity {
	return protocol.Entity{
		ID:     item.ID,
		Source: "github",
		Kind:   kind,
		Title:  item.Title,
		Repo:   item.Repo,
		URL:    item.URL,
		Status: item.Reason,
	}
}

func currentLogin(ctx context.Context) (string, error) {
	var u user
	if err := ghJSON(ctx, []string{"api", "user"}, &u); err != nil {
		return "", err
	}
	return u.Login, nil
}

func fetchNotifications(ctx context.Context) ([]notification, error) {
	var notifications []notification
	if err := ghJSON(ctx, []string{"api", "/notifications?all=true&per_page=100"}, &notifications); err != nil {
		return nil, err
	}
	return notifications, nil
}

func fetchReviewRequestedPullRequests(ctx context.Context) ([]searchPullRequest, error) {
	var prs []searchPullRequest
	args := []string{
		"search", "prs",
		"--review-requested", "@me",
		"--state", "open",
		"--limit", "100",
		"--json", "number,title,url,repository,isDraft,state",
	}
	if err := ghJSON(ctx, args, &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

func fetchAuthoredPullRequests(ctx context.Context) ([]searchPullRequest, error) {
	var prs []searchPullRequest
	args := []string{
		"search", "prs",
		"--author", "@me",
		"--state", "open",
		"--limit", "100",
		"--json", "number,title,url,repository,isDraft,state",
	}
	if err := ghJSON(ctx, args, &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

func fetchPullRequest(ctx context.Context, apiURL string) (pullRequest, error) {
	var pr pullRequest
	path, err := apiPath(apiURL)
	if err != nil {
		return pr, err
	}
	if err := ghJSON(ctx, []string{"api", path}, &pr); err != nil {
		return pr, err
	}
	return pr, nil
}

func parseOwnPullRequestID(id string) (string, int, bool) {
	value, ok := strings.CutPrefix(id, "github:own_pr:")
	if !ok {
		return "", 0, false
	}
	idx := strings.LastIndex(value, ":")
	if idx <= 0 || idx == len(value)-1 {
		return "", 0, false
	}
	number, err := strconv.Atoi(value[idx+1:])
	if err != nil {
		return "", 0, false
	}
	return value[:idx], number, true
}

func isToday(value string) bool {
	if value == "" {
		return false
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return strings.HasPrefix(value, time.Now().Format("2006-01-02"))
	}
	now := time.Now()
	y1, m1, d1 := parsed.Local().Date()
	y2, m2, d2 := now.Local().Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func reviewRequestedFrom(login string, pr pullRequest) bool {
	for _, reviewer := range pr.RequestedReviewers {
		if strings.EqualFold(reviewer.Login, login) {
			return true
		}
	}
	return false
}

func apiPath(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if parsed.Path == "" {
		return "", fmt.Errorf("missing path in url %q", raw)
	}
	return parsed.Path, nil
}

func ghJSON(ctx context.Context, args []string, target any) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "gh", args...)
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if strings.Contains(stderr, "API rate limit exceeded") || strings.Contains(stderr, "secondary rate limit") {
				until := time.Now().Add(15 * time.Minute)
				if len(args) > 0 && args[0] == "search" {
					PauseSearch(until)
				} else {
					PauseCore(until)
				}
			}
			return fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), stderr)
		}
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("decode gh output: %w", err)
	}
	return nil
}
