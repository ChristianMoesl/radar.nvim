package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"radar.nvim/internal/client"
	"radar.nvim/internal/filters"
	"radar.nvim/internal/protocol"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

type responseMsg struct {
	response protocol.Response
	err      error
}

type actionMsg struct {
	message string
	err     error
	refresh bool
}

type tickMsg time.Time

type Model struct {
	socketPath string
	response   protocol.Response
	selected   int
	loading    bool
	message    string
	err        error
	seen       map[int]bool
	seenReady  bool
}

func New(socketPath string) Model {
	return Model{socketPath: socketPath, loading: true, seen: map[int]bool{}}
}

func Run(socketPath string) error {
	_, err := tea.NewProgram(New(socketPath), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.call("tasks"), tick())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.selected > 0 {
				m.selected--
			}
		case "down", "j":
			if m.selected+1 < len(m.orderedTasks()) {
				m.selected++
			}
		case "r":
			m.loading = true
			m.message = "Refreshing..."
			return m, m.call("refresh")
		case "R":
			m.loading = true
			m.message = "Resetting..."
			return m, m.call("reset")
		case "f":
			path, err := filters.EnsureFile()
			if err != nil {
				m.err = err
				return m, nil
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			return m, tea.ExecProcess(exec.Command(editor, path), func(err error) tea.Msg {
				return actionMsg{message: "Filters updated", err: err}
			})
		case "c":
			return m, runWorkstreamCommand("create")
		case "d":
			return m, runWorkstreamCommand("delete")
		case "enter":
			tasks := m.orderedTasks()
			if len(tasks) == 0 {
				return m, nil
			}
			task := tasks[m.selected]
			return m, activate(task, m.socketPath)
		}
	case responseMsg:
		wasLoading := m.loading
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.response = msg.response
			if message := m.trackNewTasks(msg.response.Tasks); message != "" {
				m.message = message
			} else if wasLoading {
				m.message = ""
			}
			if m.selected >= len(m.orderedTasks()) {
				m.selected = max(0, len(m.orderedTasks())-1)
			}
		}
	case actionMsg:
		m.err = msg.err
		m.message = msg.message
		if msg.err == nil {
			if msg.refresh {
				return m, m.call("refresh")
			}
			return m, m.call("tasks")
		}
	case tickMsg:
		return m, tea.Batch(m.call("tasks"), tick())
	}
	return m, nil
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Radar"))
	if m.response.Summary != nil {
		s := m.response.Summary
		fmt.Fprintf(&b, "  ! %d urgent  ? %d attention  > %d progress  + %d done  - %d low",
			s.Immediate, s.Attention, s.InProgress, s.Done, s.LowPriority)
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("j/k: move  enter: open/switch  c: create  d: delete  r: refresh  R: reset  f: filters  q: close"))
	b.WriteString("\n\n")

	if len(m.response.Tasks) == 0 && !m.loading {
		b.WriteString("No tasks need your attention.\n")
	}

	index := 0
	for _, group := range groups() {
		rendered := false
		for _, task := range m.orderedTasks() {
			if task.Attention != group.key {
				continue
			}
			if !rendered {
				b.WriteString(titleStyle.Render(group.title))
				b.WriteString("\n")
				rendered = true
			}
			prefix := "  "
			line := fmt.Sprintf("%s%s  %s", prefix, task.Title, dimStyle.Render(task.Reason))
			if index == m.selected {
				line = selectedStyle.Render("> " + task.Title + "  " + task.Reason)
			}
			b.WriteString(line)
			b.WriteString("\n")
			for _, ref := range task.SourceRefs {
				fmt.Fprintf(&b, "    %s\n", dimStyle.Render(sourceRefIdentifier(ref)))
			}
			index++
		}
		if rendered {
			b.WriteString("\n")
		}
	}

	if len(m.response.Sources) > 0 {
		b.WriteString(titleStyle.Render("Sources"))
		b.WriteString("\n")
		sources := append([]protocol.SourceStatus(nil), m.response.Sources...)
		sort.SliceStable(sources, func(i, j int) bool { return sources[i].Name < sources[j].Name })
		for _, source := range sources {
			fmt.Fprintf(&b, "  %-8s %-8s %4d refs  %s\n", source.Name, source.Status, source.SourceRefCount, source.Detail)
		}
	}

	if m.loading {
		b.WriteString("\n" + dimStyle.Render(m.message))
	}
	if m.err != nil {
		b.WriteString("\n" + errorStyle.Render(m.err.Error()))
	}
	return b.String()
}

func runWorkstreamCommand(command string) tea.Cmd {
	executable, err := os.Executable()
	if err != nil {
		return func() tea.Msg { return actionMsg{err: err} }
	}
	return tea.ExecProcess(exec.Command(executable, "workstream", command), func(err error) tea.Msg {
		return actionMsg{message: "Workstream " + command + " complete", err: err, refresh: true}
	})
}

func tick() tea.Cmd {
	return tea.Tick(30*time.Second, func(now time.Time) tea.Msg {
		return tickMsg(now)
	})
}

func (m *Model) trackNewTasks(tasks []protocol.Task) string {
	next := make(map[int]bool, len(tasks))
	newTitles := make([]string, 0)
	for _, task := range tasks {
		next[task.ID] = true
		if m.seenReady && !m.seen[task.ID] {
			newTitles = append(newTitles, task.Title)
		}
	}
	m.seen = next
	m.seenReady = true
	if len(newTitles) == 0 {
		return ""
	}
	if len(newTitles) == 1 {
		return "New task: " + newTitles[0]
	}
	return fmt.Sprintf("%d new tasks", len(newTitles))
}

func (m Model) orderedTasks() []protocol.Task {
	tasks := make([]protocol.Task, 0, len(m.response.Tasks))
	for _, group := range groups() {
		for _, task := range m.response.Tasks {
			if task.Attention == group.key {
				tasks = append(tasks, task)
			}
		}
	}
	return tasks
}

func (m Model) call(method string) tea.Cmd {
	return func() tea.Msg {
		response, err := client.Call(m.socketPath, method)
		if err == nil && !response.OK {
			err = errors.New(response.Error)
		}
		return responseMsg{response: response, err: err}
	}
}

func activate(task protocol.Task, socketPath string) tea.Cmd {
	return func() tea.Msg {
		if target := tmuxTarget(task); target != "" {
			err := exec.Command("tmux", "switch-client", "-t", target).Run()
			return actionMsg{message: "Switched tmux session", err: err}
		}
		if task.ID != 0 {
			if response, err := client.Call(socketPath, fmt.Sprintf("ack:%d", task.ID)); err != nil || !response.OK {
				if err == nil {
					err = errors.New(response.Error)
				}
				return actionMsg{err: err}
			}
		}
		if task.URL == "" {
			return actionMsg{message: "Task acknowledged"}
		}
		return actionMsg{message: "Opened " + task.URL, err: openURL(task.URL)}
	}
}

func tmuxTarget(task protocol.Task) string {
	for _, ref := range task.SourceRefs {
		if ref.Source == "tmux" && ref.Kind == "session" {
			for _, key := range []string{"switch_target", "session_id", "session"} {
				if target := ref.Metadata[key]; target != "" {
					return target
				}
			}
			return ref.Title
		}
	}
	return ""
}

func openURL(url string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, url).Start()
}

func sourceRefIdentifier(ref protocol.SourceRef) string {
	for _, value := range []string{ref.ID, ref.Title, ref.Repo, ref.Path, ref.Branch} {
		if value != "" {
			return value
		}
	}
	return "unknown"
}

type group struct {
	key   string
	title string
}

func groups() []group {
	return []group{
		{key: "immediate", title: "Need immediate attention"},
		{key: "attention", title: "Need attention"},
		{key: "in_progress", title: "In progress"},
		{key: "done", title: "Done today"},
		{key: "low_priority", title: "Low priority"},
	}
}
