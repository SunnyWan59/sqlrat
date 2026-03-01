package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type DiffLineType int

const (
	DiffUnchanged DiffLineType = iota
	DiffAdded
	DiffDeleted
	DiffModified
)

type DiffLine struct {
	Type    DiffLineType
	OldText string
	NewText string
	LineNum int
}

var (
	DiffAddedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ecca3"))
	DiffDeletedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e94560"))
	DiffNormalStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#cccccc"))
	DiffLineNumStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#555555"))
)

func ComputeDiff(oldText, newText string) []DiffLine {
	oldLines := strings.Split(oldText, "\n")
	newLines := strings.Split(newText, "\n")

	type diffOp struct {
		op      string
		oldIdx  int
		newIdx  int
		content string
	}

	ops := computeEditScript(oldLines, newLines)

	var diffs []DiffLine
	lineNum := 1

	for _, op := range ops {
		switch op.op {
		case "keep":
			diffs = append(diffs, DiffLine{
				Type:    DiffUnchanged,
				OldText: op.content,
				NewText: op.content,
				LineNum: lineNum,
			})
			lineNum++
		case "delete":
			diffs = append(diffs, DiffLine{
				Type:    DiffDeleted,
				OldText: op.content,
				LineNum: lineNum,
			})
			lineNum++
		case "insert":
			diffs = append(diffs, DiffLine{
				Type:    DiffAdded,
				NewText: op.content,
				LineNum: lineNum,
			})
			lineNum++
		}
	}

	return diffs
}

func computeEditScript(a, b []string) []struct {
	op      string
	oldIdx  int
	newIdx  int
	content string
} {
	m := len(a)
	n := len(b)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	var ops []struct {
		op      string
		oldIdx  int
		newIdx  int
		content string
	}

	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			ops = append([]struct {
				op      string
				oldIdx  int
				newIdx  int
				content string
			}{{op: "keep", oldIdx: i - 1, newIdx: j - 1, content: a[i-1]}}, ops...)
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			ops = append([]struct {
				op      string
				oldIdx  int
				newIdx  int
				content string
			}{{op: "insert", oldIdx: -1, newIdx: j - 1, content: b[j-1]}}, ops...)
			j--
		} else if i > 0 {
			ops = append([]struct {
				op      string
				oldIdx  int
				newIdx  int
				content string
			}{{op: "delete", oldIdx: i - 1, newIdx: -1, content: a[i-1]}}, ops...)
			i--
		}
	}

	return ops
}

func longestCommonSubsequence(a, b []string) []string {
	m := len(a)
	n := len(b)

	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				if dp[i-1][j] > dp[i][j-1] {
					dp[i][j] = dp[i-1][j]
				} else {
					dp[i][j] = dp[i][j-1]
				}
			}
		}
	}

	var lcs []string
	i, j := m, n
	for i > 0 && j > 0 {
		if a[i-1] == b[j-1] {
			lcs = append([]string{a[i-1]}, lcs...)
			i--
			j--
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	return lcs
}

func RenderDiff(diffs []DiffLine, maxWidth int) string {
	var b strings.Builder

	for _, diff := range diffs {
		lineNumStr := DiffLineNumStyle.Render(formatLineNum(diff.LineNum))

		switch diff.Type {
		case DiffAdded:
			line := DiffAddedStyle.Render("+ " + diff.NewText)
			b.WriteString(lineNumStr + " " + line + "\n")

		case DiffDeleted:
			line := DiffDeletedStyle.Render("- " + diff.OldText)
			b.WriteString(lineNumStr + " " + line + "\n")

		case DiffUnchanged:
			line := DiffNormalStyle.Render("  " + diff.OldText)
			b.WriteString(lineNumStr + " " + line + "\n")
		}
	}

	return strings.TrimSuffix(b.String(), "\n")
}

func formatLineNum(num int) string {
	s := fmt.Sprintf("%3d", num)
	return s
}
