package tui

import (
	"strings"
	"testing"

	"radar.nvim/internal/protocol"
)

func TestViewRendersTasksAndSources(t *testing.T) {
	summary := protocol.Summary{Attention: 1}
	model := Model{
		response: protocol.Response{
			OK:      true,
			Summary: &summary,
			Tasks: []protocol.Task{{
				Title:     "Review change",
				Reason:    "review requested",
				Attention: "attention",
				SourceRefs: []protocol.SourceRef{{
					ID: "github:pr:owner/repo:1",
				}},
			}},
			Sources: []protocol.SourceStatus{{Name: "github", Status: "ok", SourceRefCount: 1}},
		},
	}

	view := model.View()
	for _, want := range []string{"Review change", "review requested", "github:pr:owner/repo:1", "github", "1 refs"} {
		if !strings.Contains(view, want) {
			t.Fatalf("View() missing %q:\n%s", want, view)
		}
	}
}

func TestTmuxTargetUsesStableSessionID(t *testing.T) {
	task := protocol.Task{SourceRefs: []protocol.SourceRef{{
		Source: "tmux",
		Kind:   "session",
		Title:  "radar",
		Metadata: map[string]string{
			"session_id":    "$3",
			"switch_target": "$3",
		},
	}}}

	if got := tmuxTarget(task); got != "$3" {
		t.Fatalf("tmuxTarget() = %q, want $3", got)
	}
}

func TestTrackNewTasksNotifiesAfterInitialLoad(t *testing.T) {
	model := New("/tmp/radar.sock")
	if got := model.trackNewTasks([]protocol.Task{{ID: 1, Title: "Existing"}}); got != "" {
		t.Fatalf("initial trackNewTasks() = %q, want empty", got)
	}
	if got := model.trackNewTasks([]protocol.Task{{ID: 1, Title: "Existing"}, {ID: 2, Title: "Review change"}}); got != "New task: Review change" {
		t.Fatalf("trackNewTasks() = %q, want new task message", got)
	}
}
