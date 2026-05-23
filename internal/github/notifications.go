package github

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"cockpit.nvim/internal/protocol"
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
	RequestedReviewers []user `json:"requested_reviewers"`
}

func FetchReviewRequests(ctx context.Context, logger *slog.Logger) ([]protocol.Item, error) {
	login, err := currentLogin(ctx)
	if err != nil {
		return nil, fmt.Errorf("get current github user: %w", err)
	}
	logger.Debug("fetched github user", "login", login)

	notifications, err := fetchNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch github notifications: %w", err)
	}
	logger.Info("fetched github notifications", "count", len(notifications))

	items := make([]protocol.Item, 0)
	for _, notification := range notifications {
		if notification.Reason != "review_requested" || notification.Subject.Type != "PullRequest" {
			continue
		}

		pr, err := fetchPullRequest(ctx, notification.Subject.URL)
		if err != nil {
			logger.Warn("could not fetch pull request for notification", "notification", notification.ID, "url", notification.Subject.URL, "error", err)
			continue
		}
		if !reviewRequestedFrom(login, pr) {
			continue
		}

		items = append(items, protocol.Item{
			ID:        "github:notification:" + notification.ID,
			Kind:      "github_review_request",
			Title:     notification.Subject.Title,
			Repo:      notification.Repository.FullName,
			URL:       pr.HTMLURL,
			Attention: "attention",
			Reason:    "review requested",
		})
	}

	logger.Info("github review requests classified", "count", len(items))
	return items, nil
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
			return fmt.Errorf("gh %s failed: %s", strings.Join(args, " "), strings.TrimSpace(string(exitErr.Stderr)))
		}
		return err
	}
	if err := json.Unmarshal(output, target); err != nil {
		return fmt.Errorf("decode gh output: %w", err)
	}
	return nil
}
