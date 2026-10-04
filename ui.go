package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	cBlue   = lipgloss.Color("#58A6FF")
	cOrange = lipgloss.Color("#F0A35E")
	cGreen  = lipgloss.Color("#56D364")
	cRed    = lipgloss.Color("#F85149")
	cDim    = lipgloss.Color("#8B949E")
	cBorder = lipgloss.Color("#30363D")
	cFg     = lipgloss.Color("#E6EDF3")
	cSelBg  = lipgloss.Color("#1C2A3D")

	sBlue   = lipgloss.NewStyle().Foreground(cBlue)
	sOrange = lipgloss.NewStyle().Foreground(cOrange)
	sGreen  = lipgloss.NewStyle().Foreground(cGreen)
	sRed    = lipgloss.NewStyle().Foreground(cRed)
	sDim    = lipgloss.NewStyle().Foreground(cDim)
	sBold   = lipgloss.NewStyle().Foreground(cFg).Bold(true)
	sBadge  = lipgloss.NewStyle().Background(cBlue).Foreground(lipgloss.Color("#0D1117")).Bold(true).Padding(0, 1)
)

type mode int

const (
	modeNormal mode = iota
	modeStart
	modeConfirm
	modeMessage
	modePath // typing a path to a local clone
)

// detail is the polled view of one job's files and git state.
type detail struct {
	plan []string
	log  []LogLine
	git  GitInfo
}

type model struct {
	cfg      Config
	gh       *GitHub
	st       *State
	assignee string
	clones   map[string]Clone

	issues  []Issue
	loading bool
	cursor  int

	agents  map[string]Agent
	stages  map[string]AgentStatus
	details map[string]detail

	mode        mode
	formClones  []Clone // ranked candidates; index len(formClones) is "another folder"
	formIdx     int
	formPause   bool
	confirmText string
	confirmDo   func() tea.Msg
	input       textinput.Model

	helpOpen  bool // full-screen help (docs/usage.md)
	helpVP    viewport.Model
	helpWidth int

	planOpen  bool // full-screen PLAN.md viewer
	planVP    viewport.Model
	planRaw   string
	planWidth int

	flash    string
	flashErr bool
	busy     string // label of a running slow action

	width, height int
}

type (
	issuesMsg struct {
		issues []Issue
		err    error
	}
	tickMsg   struct{}
	clonesMsg map[string]Clone
	pollMsg   struct {
		agents  map[string]Agent
		stages  map[string]AgentStatus
		details map[string]detail
		err     error
	}
	launchedMsg struct {
		job *Job
		err error
	}
	doneMsg struct {
		flash string
		err   error
	}
)

func newModel(cfg Config, gh *GitHub, st *State, assignee string) model {
	ti := textinput.New()
	ti.CharLimit = 2000
	return model{
		cfg: cfg, gh: gh, st: st, assignee: assignee, clones: map[string]Clone{},
		agents: map[string]Agent{}, stages: map[string]AgentStatus{}, details: map[string]detail{},
		input: ti, loading: true, formPause: cfg.pauseAfterPlan(),
	}
}

func (m model) Init() tea.Cmd {
	cfg := m.cfg
	findClones := func() tea.Msg { return clonesMsg(discoverClones(cfg)) }
	return tea.Batch(m.fetchIssues(), m.poll(), tick(), findClones)
}

func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

func (m model) fetchIssues() tea.Cmd {
	gh, query := m.gh, searchQuery(m.assignee, m.cfg.Query)
	return func() tea.Msg {
		issues, err := gh.searchIssues(query)
		return issuesMsg{issues, err}
	}
}

func (m model) poll() tea.Cmd {
	cfg := m.cfg
	jobs := map[string]Job{}
	for k, j := range m.st.Jobs {
		jobs[k] = *j
	}
	selected := ""
	if j := m.selectedJob(); j != nil {
		selected = j.key()
	}
	return func() tea.Msg {
		agents, err := cfg.listAgents()
		stages := map[string]AgentStatus{}
		details := map[string]detail{}
		for n, j := range jobs {
			if s, ok := readAgentStatus(&j); ok {
				stages[n] = s
			}
			if n != selected {
				continue
			}
			d := detail{plan: readPlan(&j)}
			sid := j.SessionID
			if a, ok := agents[j.AgentID]; ok {
				sid = a.SessionID
			}
			if sid != "" {
				if p := cfg.transcriptPath(sid); p != "" {
					d.log = readTranscript(p, j.WorktreePath, 200)
				}
			}
			if dirExists(j.workDir()) {
				d.git = gitInfo(j.workDir())
			}
			details[n] = d
		}
		return pollMsg{agents, stages, details, err}
	}
}

func (m model) selectedIssue() *Issue {
	if m.cursor < 0 || m.cursor >= len(m.issues) {
		return nil
	}
	return &m.issues[m.cursor]
}

func (m model) selectedJob() *Job {
	if i := m.selectedIssue(); i != nil {
		return m.st.Jobs[i.key()]
	}
	return nil
}

func (m *model) setFlash(s string, isErr bool) {
	m.flash, m.flashErr = s, isErr
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.planOpen {
			m.loadPlan()
		}
		if m.helpOpen {
			m.loadHelp()
		}
		return m, nil

	case issuesMsg:
		m.loading = false
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.issues = msg.issues
		if m.cursor >= len(m.issues) {
			m.cursor = max(0, len(m.issues)-1)
		}
		return m, m.poll()

	case clonesMsg:
		for name, c := range msg {
			m.clones[name] = c
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.poll(), tick())

	case pollMsg:
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
		}
		if msg.agents != nil {
			m.agents = msg.agents
		}
		m.stages = msg.stages
		for n, d := range msg.details {
			m.details[n] = d
		}
		if m.planOpen {
			m.loadPlan()
		}
		// Remember session IDs so we can resume after the agent list forgets them.
		changed := false
		for _, j := range m.st.Jobs {
			if a, ok := m.agents[j.AgentID]; ok && a.SessionID != "" && a.SessionID != j.SessionID {
				j.SessionID = a.SessionID
				changed = true
			}
		}
		if changed {
			m.st.save()
		}
		return m, nil

	case launchedMsg:
		m.busy = ""
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
			return m, nil
		}
		m.st.Jobs[msg.job.key()] = msg.job
		if err := m.st.save(); err != nil {
			m.setFlash(err.Error(), true)
		} else {
			m.setFlash(fmt.Sprintf("Started agent %s for %s in %s", msg.job.AgentID, msg.job.ref(), msg.job.WorktreePath), false)
		}
		return m, m.poll()

	case doneMsg:
		m.busy = ""
		if msg.err != nil {
			m.setFlash(msg.err.Error(), true)
		} else {
			m.setFlash(msg.flash, false)
		}
		return m, m.poll()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m model) handleKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.mode {
	case modeStart:
		switch k.String() {
		case "esc":
			m.mode = modeNormal
		case "up", "k", "left", "h", "shift+tab":
			n := len(m.formClones) + 1
			m.formIdx = (m.formIdx + n - 1) % n
		case "down", "j", "right", "l", "tab":
			m.formIdx = (m.formIdx + 1) % (len(m.formClones) + 1)
		case " ":
			m.formPause = !m.formPause
		case "enter":
			if m.formIdx == len(m.formClones) {
				m.mode = modePath
				m.input.Placeholder = "path to a local clone"
				m.input.Focus()
				return m, textinput.Blink
			}
			m.mode = modeNormal
			return m.startAgent(m.formClones[m.formIdx])
		}
		return m, nil

	case modePath:
		switch k.String() {
		case "esc":
			m.mode = modeStart
			m.input.Blur()
			m.input.Reset()
			return m, nil
		case "enter":
			path := expandHome(strings.TrimSpace(m.input.Value()))
			if abs, err := filepath.Abs(path); err == nil {
				path = abs
			}
			name := githubRemote(path)
			if name == "" {
				m.setFlash(path+" isn't a git clone with a GitHub origin remote.", true)
				return m, nil
			}
			m.input.Blur()
			m.input.Reset()
			m.mode = modeNormal
			return m.startAgent(Clone{Name: name, Path: path, Project: readProjectConfig(path)})
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd

	case modeConfirm:
		switch k.String() {
		case "y", "Y", "enter":
			m.mode = modeNormal
			m.busy = m.confirmText
			do := m.confirmDo
			return m, func() tea.Msg { return do() }
		case "n", "N", "esc":
			m.mode = modeNormal
		}
		return m, nil

	case modeMessage:
		switch k.String() {
		case "esc":
			m.mode = modeNormal
			m.input.Blur()
			return m, nil
		case "enter":
			text := strings.TrimSpace(m.input.Value())
			m.mode = modeNormal
			m.input.Blur()
			m.input.Reset()
			if text == "" {
				return m, nil
			}
			return m.sendMessage(text)
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd
	}

	if m.helpOpen {
		switch k.String() {
		case "esc", "q", "?":
			m.helpOpen = false
			return m, nil
		}
		var cmd tea.Cmd
		m.helpVP, cmd = m.helpVP.Update(k)
		return m, cmd
	}
	if k.String() == "?" {
		m.helpOpen = true
		m.helpVP = viewport.New(0, 0)
		m.helpWidth = 0
		m.loadHelp()
		return m, nil
	}

	if m.planOpen {
		switch k.String() {
		case "esc", "q", "v":
			m.planOpen = false
			return m, nil
		case "y", "m":
			// handled below, same as on the main screen
		default:
			var cmd tea.Cmd
			m.planVP, cmd = m.planVP.Update(k)
			return m, cmd
		}
	}

	job := m.selectedJob()
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, m.poll()
	case "down", "j":
		if m.cursor < len(m.issues)-1 {
			m.cursor++
		}
		return m, m.poll()
	case "r":
		m.loading = true
		return m, m.fetchIssues()
	case "enter", "s":
		if m.selectedIssue() == nil {
			return m, nil
		}
		if job != nil && k.String() == "enter" {
			return m.attach(job, false)
		}
		if job != nil && m.agentAlive(job) {
			m.setFlash("An agent is already running for this issue. Remove it first (D).", true)
			return m, nil
		}
		m.mode = modeStart
		m.formClones = rankClones(m.clones, m.selectedIssue().Repo)
		m.formIdx = 0
		return m, nil
	}

	if job == nil {
		return m, nil
	}
	switch k.String() {
	case "a":
		return m.attach(job, false)
	case "A":
		return m.attach(job, true)
	case "t":
		dir := job.workDir()
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/zsh"
		}
		c := exec.Command(shell)
		c.Dir = dir
		fmt.Printf("\nShell in %s (branch %s). Type exit to return to swat.\n\n", dir, job.Branch)
		return m, tea.ExecProcess(c, func(err error) tea.Msg { return doneMsg{"Back from shell", err} })
	case "o":
		dir := job.workDir()
		parts := m.cfg.editor()
		if err := exec.Command(parts[0], append(parts[1:], dir)...).Start(); err != nil {
			m.setFlash(err.Error(), true)
		} else {
			m.setFlash("Opened "+dir, false)
		}
		return m, nil
	case "v":
		m.planOpen = true
		m.planRaw, m.planWidth = "", 0
		m.planVP = viewport.New(0, 0)
		if !m.loadPlan() {
			m.planOpen = false
			m.setFlash("The agent hasn't written a plan yet.", true)
		}
		return m, nil
	case "y":
		if m.stages[job.key()].Stage != "awaiting_approval" {
			m.setFlash("Nothing to approve: the agent isn't waiting on a plan.", true)
			return m, nil
		}
		m.planOpen = false
		return m.sendMessage("Plan approved. Set the stage to implement and carry on.")
	case "m":
		if job.CheckedOut {
			return m, nil
		}
		m.mode = modeMessage
		m.input.Placeholder = "message to the agent"
		m.input.Focus()
		return m, textinput.Blink
	case "x":
		return m.confirm(fmt.Sprintf("Stop agent %s for %s?", job.AgentID, job.ref()), func() tea.Msg {
			return doneMsg{"Stopped " + job.AgentID, m.cfg.stop(job.AgentID)}
		})
	case "c":
		if job.CheckedOut {
			return m, nil
		}
		return m.confirm(fmt.Sprintf("Stop the agent, remove its worktree, and check out %s in %s?", job.Branch, job.Repo), func() tea.Msg {
			return m.checkout(job)
		})
	case "p":
		return m.confirm(fmt.Sprintf("Push %s to origin and open a PR page?", job.Branch), func() tea.Msg {
			return m.pushAndOpenPR(job)
		})
	case "D":
		return m.confirm(fmt.Sprintf("Delete agent %s and its worktree for %s? Unpushed commits on %s stay on the branch.", job.AgentID, job.ref(), job.Branch), func() tea.Msg {
			if !job.CheckedOut {
				if err := m.cfg.remove(job.AgentID); err != nil && dirExists(job.WorktreePath) {
					return doneMsg{err: err}
				}
			}
			delete(m.st.Jobs, job.key())
			return doneMsg{"Removed agent for " + job.ref(), m.st.save()}
		})
	}
	return m, nil
}

func (m model) agentAlive(j *Job) bool {
	a, ok := m.agents[j.AgentID]
	return ok && !j.CheckedOut && a.Status != ""
}

func (m model) confirm(text string, do func() tea.Msg) (tea.Model, tea.Cmd) {
	m.mode = modeConfirm
	m.confirmText = text
	m.confirmDo = do
	return m, nil
}

// attach opens the agent's session in a new terminal window so this screen
// stays up; inPlace (or an unknown terminal) takes over this terminal instead.
func (m model) attach(job *Job, inPlace bool) (tea.Model, tea.Cmd) {
	if _, ok := m.agents[job.AgentID]; !ok || job.CheckedOut {
		m.setFlash("This agent isn't running any more. Use t for a shell or o to open the code.", true)
		return m, nil
	}
	if !inPlace {
		cmdline := m.cfg.claudeShellCmd(job.WorktreePath, "attach", job.AgentID)
		if err := openTerminal(cmdline); err == nil {
			m.setFlash(fmt.Sprintf("Opened %s's agent in a new window", job.ref()), false)
			return m, nil
		} else if err != errUnknownTerminal {
			m.setFlash(err.Error(), true)
			return m, nil
		}
	}
	fmt.Printf("\nAttaching to %s. When this Claude window closes you will be back here.\n\n", job.AgentID)
	c := m.cfg.claude("", "attach", job.AgentID)
	return m, tea.ExecProcess(c, func(err error) tea.Msg { return doneMsg{"Detached from " + job.AgentID, err} })
}

func (m model) startAgent(clone Clone) (tea.Model, tea.Cmd) {
	issue := *m.selectedIssue()
	if _, ok := m.cfg.allRepos()[clone.Name]; !ok {
		clone.Pinned = true
		m.clones[clone.Name] = clone
		if err := m.cfg.remember(clone.Name, clone.Path); err != nil {
			m.setFlash(err.Error(), true)
		}
	}
	pause := m.formPause
	cfg, gh := m.cfg, m.gh
	m.busy = fmt.Sprintf("Starting agent for %s#%d in %s…", issue.Repo, issue.Number, clone.Name)
	return m, func() tea.Msg {
		comments, err := gh.comments(issue.Repo, issue.Number)
		if err != nil {
			return launchedMsg{err: err}
		}
		job, err := cfg.launch(clone, issue, comments, pause)
		return launchedMsg{job, err}
	}
}

func (m model) sendMessage(text string) (tea.Model, tea.Cmd) {
	job := m.selectedJob()
	if a, ok := m.agents[job.AgentID]; ok && a.Status == "busy" {
		m.setFlash("The agent is busy. Attach (a) to talk to it directly, or wait until it's idle.", true)
		return m, nil
	}
	if job.SessionID == "" {
		m.setFlash("No session ID recorded for this agent yet.", true)
		return m, nil
	}
	cfg, st := m.cfg, m.st
	m.busy = "Sending message…"
	return m, func() tea.Msg {
		id, err := cfg.message(job, job.SessionID, text)
		if err != nil {
			return doneMsg{err: err}
		}
		job.AgentID = id
		return doneMsg{"Message sent to " + id, st.save()}
	}
}

// checkout frees the branch from its worktree and checks it out in the main
// clone, keeping a copy of the agent's progress folder so the plan and
// summary stay visible.
func (m model) checkout(job *Job) tea.Msg {
	if a, ok := m.agents[job.AgentID]; ok && a.Status != "" {
		m.cfg.stop(job.AgentID)
	}
	after := *job
	after.CheckedOut = true
	save := after.dir()
	os.RemoveAll(save)
	os.MkdirAll(save, 0o755)
	if src := filepath.Join(job.WorktreePath, job.StateDir); dirExists(src) {
		exec.Command("cp", "-R", src, save).Run()
	}
	git(job.RepoPath, "worktree", "unlock", job.WorktreePath)
	if err := git(job.RepoPath, "worktree", "remove", job.WorktreePath); err != nil {
		return doneMsg{err: fmt.Errorf("%v (commit or discard the worktree's changes first)", err)}
	}
	if err := git(job.RepoPath, "checkout", job.Branch); err != nil {
		return doneMsg{err: fmt.Errorf("worktree removed, but checkout failed: %v", err)}
	}
	job.CheckedOut = true
	return doneMsg{fmt.Sprintf("Checked out %s in %s", job.Branch, job.RepoPath), m.st.save()}
}

func (m model) pushAndOpenPR(job *Job) tea.Msg {
	if err := git(job.workDir(), "push", "-u", "origin", job.Branch); err != nil {
		return doneMsg{err: err}
	}
	url := fmt.Sprintf("https://github.com/%s/compare/%s?expand=1", job.Repo, job.Branch)
	return doneMsg{"Pushed " + job.Branch, openURL(url)}
}

// ---------------- view ----------------

var stageOrder = []string{"research", "plan", "implement", "ready"}

func stageLabel(s string) string {
	switch s {
	case "awaiting_approval":
		return "plan ready"
	case "implement":
		return "impl"
	}
	return s
}

// statusCell is the colored status shown in the issue list.
func (m model) statusCell(n string) string {
	job := m.st.Jobs[n]
	if job == nil {
		return sDim.Render("○ idle")
	}
	if job.CheckedOut {
		return sGreen.Render("✓ checked out")
	}
	st := m.stages[n]
	a, alive := m.agents[job.AgentID]
	switch st.Stage {
	case "awaiting_approval":
		return sOrange.Render("◆ plan ready")
	case "blocked":
		return sOrange.Render("◆ blocked")
	case "ready":
		return sGreen.Render("✓ ready")
	}
	stage := st.Stage
	if stage == "" {
		stage = "starting"
	}
	switch {
	case alive && a.Status == "busy":
		return sBlue.Render(fmt.Sprintf("● %s %s", stageLabel(stage), shortDur(time.Since(job.StartedAt))))
	case alive && a.Status != "":
		return sOrange.Render("◆ idle · " + stageLabel(stage))
	default:
		return sDim.Render("✕ stopped")
	}
}

func shortDur(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	return fmt.Sprintf("%dh%02dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m model) View() string {
	if m.width == 0 {
		return ""
	}
	header := m.viewHeader()
	footer := m.viewFooter()
	flash := m.viewFlash()
	bodyH := m.height - 3
	leftW := max(44, m.width*42/100)
	rightW := m.width - leftW - 1
	var body string
	if m.helpOpen {
		title := "Help · more guides: swat help"
		if m.helpVP.TotalLineCount() > m.helpVP.Height {
			title += fmt.Sprintf(" · %d%%", int(m.helpVP.ScrollPercent()*100))
		}
		body = box(title, strings.Split(m.helpVP.View(), "\n"), m.width, bodyH, cBlue)
	} else if m.planOpen {
		body = m.viewPlan(m.width, bodyH)
	} else {
		left := m.viewLeft(leftW, bodyH)
		right := m.viewRight(rightW, bodyH)
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, flash, footer)
}

func (m model) viewHeader() string {
	running, needs, idle := 0, 0, 0
	for _, i := range m.issues {
		cell := ansi.Strip(m.statusCell(i.key()))
		switch {
		case strings.HasPrefix(cell, "●"):
			running++
		case strings.HasPrefix(cell, "◆"):
			needs++
		case strings.HasPrefix(cell, "○"):
			idle++
		}
	}
	parts := []string{sBadge.Render("swat"), sDim.Render("@" + m.assignee)}
	if m.cfg.Query != "" {
		parts = append(parts, sDim.Render(m.cfg.Query))
	}
	if d := m.cfg.ClaudeConfigDir; d != "" {
		parts = append(parts, sDim.Render("CLAUDE_CONFIG_DIR="+d))
	}
	leftPart := strings.Join(parts, "  ")
	rightPart := sBlue.Render(fmt.Sprintf("● %d running", running)) + "  " +
		sOrange.Render(fmt.Sprintf("◆ %d needs you", needs)) + "  " +
		sDim.Render(fmt.Sprintf("○ %d idle", idle))
	gap := m.width - lipgloss.Width(leftPart) - lipgloss.Width(rightPart)
	if gap < 1 {
		return ansi.Truncate(leftPart, m.width, "…")
	}
	return leftPart + strings.Repeat(" ", gap) + rightPart
}

func (m model) viewFlash() string {
	switch {
	case m.mode == modeConfirm:
		return sOrange.Render(ansi.Truncate(m.confirmText+"  [y/n]", m.width, "…"))
	case m.mode == modeMessage:
		return sBlue.Render("message › ") + m.input.View()
	case m.mode == modePath:
		return sBlue.Render("folder › ") + m.input.View()
	case m.busy != "":
		return sBlue.Render(ansi.Truncate(m.busy, m.width, "…"))
	case m.flash != "":
		st := sGreen
		if m.flashErr {
			st = sRed
		}
		return st.Render(ansi.Truncate(m.flash, m.width, "…"))
	}
	return ""
}

func (m model) viewFooter() string {
	var keys [][2]string
	switch m.mode {
	case modeStart:
		keys = [][2]string{{"↑↓", "repo"}, {"space", "pause after plan"}, {"enter", "launch"}, {"esc", "cancel"}}
	case modeConfirm:
		keys = [][2]string{{"y", "confirm"}, {"n", "cancel"}}
	case modeMessage, modePath:
		keys = [][2]string{{"enter", "ok"}, {"esc", "cancel"}}
	default:
		if m.helpOpen {
			keys = [][2]string{{"↑↓/pgup/pgdn", "scroll"}, {"esc", "back"}}
			break
		}
		if m.planOpen {
			keys = [][2]string{{"↑↓/pgup/pgdn", "scroll"}, {"y", "approve plan"}, {"m", "request changes"}, {"esc", "back"}}
			break
		}
		keys = [][2]string{{"↑↓", "move"}, {"s", "start"}, {"enter/a", "open agent"}, {"A", "attach here"}, {"t", "shell"}, {"o", "editor"}, {"c", "checkout"}, {"y", "approve"}, {"m", "message"}, {"x", "stop"}, {"p", "push+PR"}, {"D", "delete"}, {"r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	}
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = sBlue.Render(k[0]) + " " + k[1]
	}
	return ansi.Truncate(strings.Join(parts, "  "), m.width, "…")
}

func (m model) viewLeft(w, h int) string {
	var form []string
	if m.mode == modeStart || m.mode == modePath {
		form = m.startForm()
	}
	formH := 0
	if form != nil {
		formH = len(form) + 2
	}
	var rows []string
	switch {
	case m.loading && len(m.issues) == 0:
		rows = []string{sDim.Render("Loading issues…")}
	case len(m.issues) == 0:
		rows = []string{sDim.Render("No open issues assigned to you."), sDim.Render("Run `swat doctor` if that looks wrong.")}
	}
	inner := w - 4
	for idx, i := range m.issues {
		cursor := "  "
		if idx == m.cursor {
			cursor = sBlue.Render("▸ ")
		}
		status := m.statusCell(i.key())
		status += strings.Repeat(" ", max(1, 17-lipgloss.Width(status)))
		num := fmt.Sprintf("%-7s", fmt.Sprintf("#%d", i.Number))
		titleW := inner - 2 - 7 - 17
		line1 := cursor + num + status + ansi.Truncate(i.Title, max(4, titleW), "…")
		sub := i.Repo
		if j := m.st.Jobs[i.key()]; j != nil && !strings.EqualFold(j.Repo, i.Repo) {
			sub += " → " + j.Repo
		}
		if labels := i.labelNames(); len(labels) > 0 {
			sub += " · " + strings.Join(labels, " · ")
		}
		line2 := "         " + sDim.Render(ansi.Truncate(sub, max(4, inner-9), "…"))
		if idx == m.cursor {
			hl := lipgloss.NewStyle().Background(cSelBg).Width(inner)
			line1, line2 = hl.Render(line1), hl.Render(line2)
		}
		rows = append(rows, line1, line2)
	}
	// Keep the cursor visible when the list is longer than the box.
	listH := h - formH - 2
	if start := (m.cursor+1)*2 - listH; start > 0 && start < len(rows) {
		rows = rows[start:]
	}
	list := box(fmt.Sprintf("Issues (%d)", len(m.issues)), rows, w, h-formH, cBlue)
	if form == nil {
		return list
	}
	issue := m.selectedIssue()
	return lipgloss.JoinVertical(lipgloss.Left, list, box(fmt.Sprintf("Start agent · %s#%d", issue.Repo, issue.Number), form, w, formH, cBorder))
}

// startForm lists where the fix can be made (a window of the ranked local
// clones plus "another folder") and the launch options.
func (m model) startForm() []string {
	home, _ := os.UserHomeDir()
	const visible = 5
	n := len(m.formClones) + 1
	first := max(0, min(m.formIdx-visible/2, n-visible))
	lines := []string{sDim.Render("Fix it in:")}
	if len(m.formClones) == 0 {
		lines = append(lines, sDim.Render("  no local clones found (see swat doctor)"))
	}
	for i := first; i < min(n, first+visible); i++ {
		mark := sDim.Render("  ( ) ")
		if i == m.formIdx {
			mark = sBlue.Render("  (•) ")
		}
		if i == len(m.formClones) {
			lines = append(lines, mark+"another folder…")
			continue
		}
		c := m.formClones[i]
		lines = append(lines, mark+c.Name+"  "+sDim.Render(strings.Replace(c.Path, home, "~", 1)))
	}
	pause := sDim.Render("[ ]")
	if m.formPause {
		pause = sBlue.Render("[x]")
	}
	modelName, perms := m.cfg.Model, m.cfg.PermissionMode
	if modelName == "" {
		modelName = "default"
	}
	if perms == "" {
		perms = "default"
	}
	return append(lines,
		"",
		pause+" pause for my approval after the plan",
		sDim.Render("model ")+modelName+sDim.Render("   permissions ")+perms,
	)
}

func (m model) viewRight(w, h int) string {
	issue := m.selectedIssue()
	if issue == nil {
		return box("", nil, w, h, cBorder)
	}
	job := m.st.Jobs[issue.key()]
	title := fmt.Sprintf("%s#%d · %s", issue.Repo, issue.Number, issue.Title)
	if job == nil {
		body := []string{
			sDim.Render("No agent yet. Press s to start one."),
			"",
			sDim.Render("repo    ") + issue.Repo,
			sDim.Render("labels  ") + strings.Join(issue.labelNames(), ", "),
			sDim.Render("updated ") + issue.UpdatedAt.Local().Format("Jan 2, 2006"),
			sDim.Render("url     ") + issue.HTMLURL,
			"",
		}
		wrapped := lipgloss.NewStyle().Width(w - 4).Render(strings.ReplaceAll(strings.TrimSpace(issue.Body), "\r", ""))
		body = append(body, strings.Split(wrapped, "\n")...)
		return box(title, body, w, h, cBorder)
	}

	d := m.details[issue.key()]
	st := m.stages[issue.key()]
	a, alive := m.agents[job.AgentID]

	agentLine := job.AgentID
	switch {
	case job.CheckedOut:
		agentLine += sDim.Render("  removed (branch checked out)")
	case alive && a.Status == "":
		agentLine += sDim.Render("  stopped · m to wake it with a message")
	case alive:
		agentLine += sDim.Render(fmt.Sprintf("  %s · %s", a.Status, a.State))
	default:
		agentLine += sDim.Render("  not in claude agents")
	}
	wt := job.WorktreePath
	if job.CheckedOut {
		wt = job.RepoPath + sDim.Render("  (main clone)")
	}
	info := []string{
		m.pipeline(job, st.Stage),
		"",
		sDim.Render("repo     ") + job.Repo,
		sDim.Render("branch   ") + job.Branch + sDim.Render(fmt.Sprintf("   %d commits ahead", d.git.Ahead)),
		sDim.Render("worktree ") + wt,
		sDim.Render("diff     ") + orDash(d.git.ShortStat),
		sDim.Render("agent    ") + agentLine,
	}
	if st.Note != "" {
		info = append(info, sDim.Render("note     ")+sOrange.Render(st.Note))
	}
	infoH := len(info) + 2

	plan := make([]string, 0, len(d.plan))
	for _, p := range d.plan {
		switch {
		case strings.HasPrefix(p, "- [x]"), strings.HasPrefix(p, "- [X]"):
			plan = append(plan, sGreen.Render("[x]")+p[5:])
		case strings.HasPrefix(p, "- [ ]"):
			plan = append(plan, sDim.Render("[ ]"+p[5:]))
		default:
			plan = append(plan, p)
		}
	}
	if len(plan) == 0 {
		plan = []string{sDim.Render("No plan yet.")}
	}
	planH := min(len(plan)+2, max(4, (h-infoH)/2))

	planTitle, planBorder := "Plan · v to read", cBorder
	if st.Stage == "awaiting_approval" {
		planTitle, planBorder = "Plan ready for review · v to read · y to approve", cOrange
	}

	logH := h - infoH - planH
	var logLines []string
	for _, l := range d.log {
		text := l.Text
		if l.Kind == "tool" {
			if name, rest, ok := strings.Cut(text, " "); ok {
				text = sBlue.Render(name) + " " + rest
			} else {
				text = sBlue.Render(text)
			}
		}
		logLines = append(logLines, sDim.Render(l.At.Local().Format("15:04"))+"  "+text)
	}
	if len(logLines) == 0 {
		logLines = []string{sDim.Render("Waiting for the agent's first steps…")}
	}
	if over := len(logLines) - (logH - 2); over > 0 {
		logLines = logLines[over:]
	}
	logTitle := "Log"
	if alive && a.Status == "busy" {
		logTitle = "Log · live"
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		box(title, info, w, infoH, cBorder),
		box(planTitle, plan, w, planH, planBorder),
		box(logTitle, logLines, w, logH, cBorder),
	)
}

func (m model) pipeline(job *Job, stage string) string {
	cur := -1
	for i, s := range stageOrder {
		if s == stage {
			cur = i
		}
	}
	if stage == "awaiting_approval" {
		cur = 1
	}
	if job.CheckedOut && cur < 0 {
		cur = len(stageOrder) - 1
	}
	parts := make([]string, len(stageOrder))
	for i, s := range stageOrder {
		label := s
		if s == "ready" {
			label = "ready to test"
		}
		switch {
		case i < cur, i == cur && s == "ready":
			parts[i] = sGreen.Render("✓ " + label)
		case i == cur && (stage == "awaiting_approval" || stage == "blocked"):
			parts[i] = sOrange.Render("◆ " + label + " (needs you)")
		case i == cur:
			parts[i] = sBlue.Bold(true).Render("● " + label)
		default:
			parts[i] = sDim.Render("○ " + label)
		}
	}
	if stage == "blocked" && cur < 0 {
		return sOrange.Render("◆ blocked (needs you)")
	}
	return strings.Join(parts, sDim.Render("  ──  "))
}

func orDash(s string) string {
	if s == "" {
		return sDim.Render("—")
	}
	return s
}

// box draws a rounded border with a title in the top edge, w×h cells.
func box(title string, body []string, w, h int, border lipgloss.Color) string {
	if h < 2 || w < 6 {
		return ""
	}
	bs := lipgloss.NewStyle().Foreground(border)
	inner := w - 2
	title = ansi.Truncate(title, inner-4, "…")
	top := bs.Render("╭─")
	if title != "" {
		top += " " + sBold.Render(title) + " "
	}
	top += bs.Render(strings.Repeat("─", max(0, w-1-lipgloss.Width(top))) + "╮")
	lines := []string{top}
	for i := 0; i < h-2; i++ {
		content := ""
		if i < len(body) {
			content = ansi.Truncate(body[i], inner-2, "…")
		}
		pad := strings.Repeat(" ", max(0, inner-2-lipgloss.Width(content)))
		lines = append(lines, bs.Render("│")+" "+content+pad+" "+bs.Render("│"))
	}
	lines = append(lines, bs.Render("╰"+strings.Repeat("─", inner)+"╯"))
	return strings.Join(lines, "\n")
}

// loadPlan (re)reads the selected job's PLAN.md into the viewer, rendering the
// markdown only when the file or width changed. Reports whether a plan exists.
func (m *model) loadPlan() bool {
	job := m.selectedJob()
	if job == nil {
		return false
	}
	data, err := os.ReadFile(filepath.Join(job.dir(), job.StateDir, "PLAN.md"))
	if err != nil {
		return false
	}
	w, h := m.width-4, m.height-5
	m.planVP.Width, m.planVP.Height = w, h
	if string(data) == m.planRaw && w == m.planWidth {
		return true
	}
	m.planRaw, m.planWidth = string(data), w
	out := string(data)
	if r, err := glamour.NewTermRenderer(glamour.WithStandardStyle("dark"), glamour.WithWordWrap(w-2)); err == nil {
		if rendered, err := r.Render(out); err == nil {
			out = rendered
		}
	}
	offset := m.planVP.YOffset
	m.planVP.SetContent(strings.TrimRight(out, "\n"))
	m.planVP.SetYOffset(offset)
	return true
}

func (m model) viewPlan(w, h int) string {
	job := m.selectedJob()
	issue := m.selectedIssue()
	title := fmt.Sprintf("PLAN.md · %s#%d · %s", issue.Repo, issue.Number, issue.Title)
	border := cBorder
	if m.stages[job.key()].Stage == "awaiting_approval" {
		title = "Ready for review · " + title
		border = cOrange
	}
	if pct := m.planVP.ScrollPercent(); m.planVP.TotalLineCount() > m.planVP.Height {
		title += fmt.Sprintf(" · %d%%", int(pct*100))
	}
	return box(title, strings.Split(m.planVP.View(), "\n"), w, h, border)
}

// loadHelp renders the usage guide into the help viewer for the current width.
func (m *model) loadHelp() {
	w, h := m.width-4, m.height-5
	m.helpVP.Width, m.helpVP.Height = w, h
	if w == m.helpWidth {
		return
	}
	m.helpWidth = w
	m.helpVP.SetContent(strings.TrimRight(renderMarkdown(docText("usage.md"), w), "\n"))
}
