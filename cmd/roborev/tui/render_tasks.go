package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/roborev-dev/roborev/internal/storage"
)

func (m model) renderTasksView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("roborev tasks (background fixes)"))
	b.WriteString("\x1b[K\n")

	// Help overlay
	if m.fixShowHelp {
		return m.renderTasksHelpOverlay(&b)
	}

	if len(m.fixJobs) == 0 {
		b.WriteString("\n  No fix tasks. Press F on a review to trigger a background fix.\n")
		b.WriteString("\n")
		b.WriteString(renderHelpTable([][]helpItem{
			{{"T", "back to queue"}, {"F", "fix review"}, {"q", "quit"}},
		}, m.width))
		b.WriteString("\x1b[K\x1b[J")
		return b.String()
	}

	// Column layout: status, job, parent, queued, elapsed are fixed.
	// Branch/repo/ref+subject split remaining space.
	const statusW = 8                                                                  // "canceled" is the longest
	const idW = 5                                                                      // "#" + 4-digit number
	const parentW = 11                                                                 // "fixes #NNNN"
	const queuedW = 12                                                                 // "Jan 02 15:04"
	const elapsedW = 8                                                                 // "59m59s"
	fixedW := 2 + statusW + 1 + idW + 1 + parentW + 1 + queuedW + 1 + elapsedW + 1 + 1 // prefix + inter-column spaces
	flexW := max(m.width-fixedW, 20)
	branchW := max(1, flexW*20/100)
	repoW := max(1, flexW*24/100)
	refSubjectW := max(1, flexW-branchW-repoW-2)

	// Header
	header := fmt.Sprintf("  %-*s %-*s %-*s %-*s %*s %-*s %-*s %-*s",
		statusW, "Status", idW, "Job", parentW, "Parent",
		queuedW, "Queued", elapsedW, "Elapsed",
		branchW, "Branch", repoW, "Repo", refSubjectW, "Ref/Subject")
	b.WriteString(statusStyle.Render(header))
	b.WriteString("\x1b[K\n")
	b.WriteString("  " + strings.Repeat("-", min(m.width-4, 200)))
	b.WriteString("\x1b[K\n")

	// Render each fix job
	tasksHelpRows := [][]helpItem{
		{{"enter", "view"}, {"P", "parent"}, {"p", "patch"}, {"A", "apply"}, {"l", "log"}, {"x", "cancel"}, {"?", "help"}, {"T/esc", "back"}},
	}
	tasksHelpLines := len(reflowHelpRows(tasksHelpRows, m.width))
	visibleRows := m.height - (6 + tasksHelpLines) // title + header + separator + status + scroll + help(N)
	visibleRows = max(visibleRows, 1)
	startIdx := 0
	if m.fixSelectedIdx >= visibleRows {
		startIdx = m.fixSelectedIdx - visibleRows + 1
	}

	for i := startIdx; i < len(m.fixJobs) && i < startIdx+visibleRows; i++ {
		job := m.fixJobs[i]

		// Status label
		var statusLabel string
		var statusStyle lipgloss.Style
		switch job.Status {
		case storage.JobStatusQueued:
			statusLabel = "queued"
			statusStyle = queuedStyle
		case storage.JobStatusRunning:
			statusLabel = "running"
			statusStyle = runningStyle
		case storage.JobStatusDone:
			statusLabel = "ready"
			statusStyle = doneStyle
		case storage.JobStatusFailed:
			statusLabel = "failed"
			statusStyle = failedStyle
		case storage.JobStatusCanceled:
			statusLabel = "canceled"
			statusStyle = canceledStyle
		case storage.JobStatusApplied:
			statusLabel = "applied"
			statusStyle = doneStyle
		case storage.JobStatusRebased:
			statusLabel = "rebased"
			statusStyle = canceledStyle
		}

		parentRef := ""
		if job.ParentJobID != nil {
			parentRef = fmt.Sprintf("fixes #%d", *job.ParentJobID)
		}
		queued := ""
		if !job.EnqueuedAt.IsZero() {
			queued = job.EnqueuedAt.Local().Format("Jan 02 15:04")
		}
		elapsed := ""
		if job.StartedAt != nil {
			if job.FinishedAt != nil {
				elapsed = job.FinishedAt.Sub(*job.StartedAt).Round(time.Second).String()
			} else {
				elapsed = time.Since(*job.StartedAt).Round(time.Second).String()
			}
		}
		branch := truncateString(job.Branch, branchW)
		defaultRepoName := job.RepoName
		if defaultRepoName == "" && job.RepoPath != "" {
			defaultRepoName = filepath.Base(job.RepoPath)
		}
		repo := m.getDisplayName(job.RepoPath, defaultRepoName)
		if m.status.MachineID != "" && job.SourceMachineID != "" && job.SourceMachineID != m.status.MachineID {
			repo += " [R]"
		}
		repo = truncateString(repo, repoW)

		refSubject := job.GitRef
		if job.CommitSubject != "" {
			if refSubject != "" {
				refSubject += " "
			}
			refSubject += job.CommitSubject
		}
		refSubject = truncateString(refSubject, refSubjectW)

		if i == m.fixSelectedIdx {
			line := fmt.Sprintf("  %-*s #%-4d %-*s %-*s %*s %-*s %-*s %-*s",
				statusW, statusLabel, job.ID, parentW, parentRef,
				queuedW, queued, elapsedW, elapsed,
				branchW, branch, repoW, repo, refSubjectW, refSubject)
			b.WriteString(selectedStyle.Render(line))
		} else {
			styledStatus := statusStyle.Render(fmt.Sprintf("%-*s", statusW, statusLabel))
			rest := fmt.Sprintf(" #%-4d %-*s %-*s %*s %-*s %-*s %-*s",
				job.ID, parentW, parentRef,
				queuedW, queued, elapsedW, elapsed,
				branchW, branch, repoW, repo, refSubjectW, refSubject)
			b.WriteString("  " + styledStatus + rest)
		}
		b.WriteString("\x1b[K\n")
	}

	// Flash message
	if m.flashMessage != "" && time.Now().Before(m.flashExpiresAt) && m.flashView == viewTasks {
		flashStyle := lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "28", Dark: "46"})
		b.WriteString(flashStyle.Render(m.flashMessage))
	}
	b.WriteString("\x1b[K\n")

	// Help
	b.WriteString(renderHelpTable(tasksHelpRows, m.width))
	b.WriteString("\x1b[K\x1b[J")

	return b.String()
}
func (m model) renderTasksHelpOverlay(b *strings.Builder) string {
	help := []string{
		"",
		"  Task Status",
		"    queued     Waiting for a worker to pick up the job",
		"    running    Agent is working in an isolated worktree",
		"    ready      Patch captured and ready to apply to your working tree",
		"    failed     Agent failed (press enter or l to see error details)",
		"    applied    Patch was applied and committed to your working tree",
		"    canceled   Job was canceled by user",
		"",
		"  Keybindings",
		"    enter/l    View review output (ready) or error (failed) or log (running)",
		"    P          Open the parent review for this fix task",
		"    p          View the patch diff for a ready job",
		"    A          Apply patch from a ready job to your working tree",
		"    R          Re-run fix against current HEAD (when patch is stale)",
		"    F          Trigger a new fix from a review (from queue view)",
		"    x          Cancel a queued or running job",
		"    T/esc      Return to the main queue view",
		"    ?          Toggle this help",
		"",
		"  Workflow",
		"    1. Press F on a failing review to trigger a background fix",
		"    2. The agent runs in an isolated worktree (your files are untouched)",
		"    3. When status shows 'ready', press A to apply and commit the patch",
		"    4. If the patch is stale (code changed since), press R to re-run",
		"",
	}
	for _, line := range help {
		b.WriteString(line)
		b.WriteString("\x1b[K\n")
	}
	b.WriteString(helpStyle.Render("?: close help"))
	b.WriteString("\x1b[K\x1b[J")
	return b.String()
}
func (m model) renderPatchView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("patch for fix job #%d", m.patchJobID)))
	b.WriteString("\x1b[K\n")

	if m.patchText == "" {
		b.WriteString("\n  No patch available.\n")
	} else {
		lines := strings.Split(m.patchText, "\n")
		visibleRows := max(m.height-4, 1)
		maxScroll := max(len(lines)-visibleRows, 0)
		start := max(min(m.patchScroll, maxScroll), 0)
		end := min(start+visibleRows, len(lines))

		addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("34"))   // green
		delStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("160"))  // red
		hdrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("33"))   // blue
		metaStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245")) // gray

		for _, line := range lines[start:end] {
			display := line
			switch {
			case strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++"):
				display = addStyle.Render(line)
			case strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---"):
				display = delStyle.Render(line)
			case strings.HasPrefix(line, "@@"):
				display = hdrStyle.Render(line)
			case strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") ||
				strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
				display = metaStyle.Render(line)
			}
			b.WriteString("  " + display)
			b.WriteString("\x1b[K\n")
		}

		if len(lines) > visibleRows {
			pct := 0
			if maxScroll > 0 {
				pct = start * 100 / maxScroll
			}
			b.WriteString(helpStyle.Render(fmt.Sprintf("  [%d%%]", pct)))
			b.WriteString("\x1b[K\n")
		}
	}

	b.WriteString(renderHelpTable([][]helpItem{
		{{"j/k/↑/↓", "scroll"}, {"esc", "back to tasks"}},
	}, m.width))
	b.WriteString("\x1b[K\x1b[J")
	return b.String()
}
func (m model) renderWorktreeConfirmView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Create Worktree"))
	b.WriteString("\x1b[K\n\n")

	fmt.Fprintf(&b, "  Branch %q is not checked out anywhere.\n", m.worktreeConfirmBranch)
	b.WriteString("  Create a temporary worktree to apply and commit the patch?\n\n")
	b.WriteString("  The worktree will be removed after the commit.\n")
	b.WriteString("  The commit will persist on the branch.\n\n")

	b.WriteString(helpStyle.Render("y/enter: create worktree and apply | esc/n: cancel"))
	b.WriteString("\x1b[K\x1b[J")

	return b.String()
}
