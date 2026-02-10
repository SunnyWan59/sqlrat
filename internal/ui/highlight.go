package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	KeywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c678dd"))
	FunctionStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#61afef"))
	StringStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#98c379"))
	NumberStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#d19a66"))
	CommentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#5c6370")).Italic(true)
	OperatorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#56b6c2"))
)

var sqlKeywords = []string{
	"SELECT", "FROM", "WHERE", "INSERT", "INTO", "VALUES", "UPDATE", "SET",
	"DELETE", "CREATE", "TABLE", "DROP", "ALTER", "ADD", "COLUMN",
	"PRIMARY", "KEY", "FOREIGN", "REFERENCES", "INDEX", "UNIQUE",
	"NOT", "NULL", "DEFAULT", "AUTO_INCREMENT", "CONSTRAINT",
	"JOIN", "INNER", "LEFT", "RIGHT", "OUTER", "FULL", "CROSS", "ON",
	"AND", "OR", "IN", "BETWEEN", "LIKE", "IS", "AS", "DISTINCT",
	"ORDER", "BY", "GROUP", "HAVING", "LIMIT", "OFFSET",
	"UNION", "ALL", "INTERSECT", "EXCEPT",
	"CASE", "WHEN", "THEN", "ELSE", "END",
	"BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION",
	"GRANT", "REVOKE", "PRIVILEGE",
	"VIEW", "TRIGGER", "PROCEDURE", "FUNCTION",
	"EXECUTE", "EXEC", "CALL",
	"IF", "EXISTS",
	"SERIAL", "BIGSERIAL", "SMALLSERIAL",
	"INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT",
	"DECIMAL", "NUMERIC", "FLOAT", "DOUBLE", "REAL",
	"CHAR", "VARCHAR", "TEXT", "BLOB",
	"DATE", "TIME", "TIMESTAMP", "DATETIME",
	"BOOLEAN", "BOOL",
	"ARRAY", "JSON", "JSONB", "UUID",
	"CASCADE", "RESTRICT", "NO", "ACTION",
	"ASC", "DESC",
	"TRUE", "FALSE",
	"WITH", "RECURSIVE",
	"RETURNING",
	"TEMP", "TEMPORARY", "UNLOGGED",
	"PARTITION", "PARTITIONED",
	"ANALYZE", "EXPLAIN", "VACUUM",
}

var sqlFunctions = []string{
	"COUNT", "SUM", "AVG", "MIN", "MAX",
	"CONCAT", "SUBSTRING", "LENGTH", "UPPER", "LOWER", "TRIM",
	"COALESCE", "NULLIF", "CAST",
	"NOW", "CURRENT_DATE", "CURRENT_TIME", "CURRENT_TIMESTAMP",
	"EXTRACT", "DATE_PART", "TO_CHAR",
	"ROW_NUMBER", "RANK", "DENSE_RANK", "LAG", "LEAD",
	"FIRST_VALUE", "LAST_VALUE",
	"STRING_AGG", "ARRAY_AGG",
}

var majorClauses = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "SET": true,
	"HAVING": true, "RETURNING": true, "VALUES": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
}

var indentClauses = map[string]bool{
	"AND": true, "OR": true,
}

var joinKeywords = map[string]bool{
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"FULL": true, "CROSS": true, "OUTER": true,
}

func FormatSQL(sql string) string {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return sql
	}

	keywordSet := make(map[string]bool)
	for _, kw := range sqlKeywords {
		keywordSet[kw] = true
	}
	for _, fn := range sqlFunctions {
		keywordSet[fn] = true
	}

	type segment struct {
		text    string
		isToken bool
	}

	var segments []segment
	i := 0
	for i < len(sql) {
		if sql[i] == '-' && i+1 < len(sql) && sql[i+1] == '-' {
			end := strings.Index(sql[i:], "\n")
			if end == -1 {
				segments = append(segments, segment{text: sql[i:], isToken: false})
				i = len(sql)
			} else {
				segments = append(segments, segment{text: sql[i : i+end], isToken: false})
				i = i + end
			}
			continue
		}

		if sql[i] == '\'' {
			end := i + 1
			for end < len(sql) {
				if sql[end] == '\\' {
					end += 2
					continue
				}
				if sql[end] == '\'' {
					end++
					break
				}
				end++
			}
			segments = append(segments, segment{text: sql[i:end], isToken: false})
			i = end
			continue
		}

		if sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r' {
			for i < len(sql) && (sql[i] == ' ' || sql[i] == '\t' || sql[i] == '\n' || sql[i] == '\r') {
				i++
			}
			segments = append(segments, segment{text: " ", isToken: false})
			continue
		}

		if (sql[i] >= 'a' && sql[i] <= 'z') || (sql[i] >= 'A' && sql[i] <= 'Z') || sql[i] == '_' {
			end := i + 1
			for end < len(sql) && ((sql[end] >= 'a' && sql[end] <= 'z') || (sql[end] >= 'A' && sql[end] <= 'Z') || (sql[end] >= '0' && sql[end] <= '9') || sql[end] == '_') {
				end++
			}
			word := sql[i:end]
			upper := strings.ToUpper(word)
			if keywordSet[upper] {
				segments = append(segments, segment{text: upper, isToken: true})
			} else {
				segments = append(segments, segment{text: word, isToken: true})
			}
			i = end
			continue
		}

		if sql[i] == '(' || sql[i] == ')' || sql[i] == ',' || sql[i] == ';' || sql[i] == '.' || sql[i] == '*' {
			segments = append(segments, segment{text: string(sql[i]), isToken: false})
			i++
			continue
		}

		end := i + 1
		for end < len(sql) && !((sql[end] >= 'a' && sql[end] <= 'z') || (sql[end] >= 'A' && sql[end] <= 'Z') || sql[end] == '_' || sql[end] == ' ' || sql[end] == '\t' || sql[end] == '\n' || sql[end] == '\r' || sql[end] == '\'' || sql[end] == '(' || sql[end] == ')' || sql[end] == ',' || sql[end] == ';' || sql[end] == '.' || sql[end] == '*' || (sql[end] >= '0' && sql[end] <= '9')) {
			end++
		}
		segments = append(segments, segment{text: sql[i:end], isToken: false})
		i = end
	}

	var result strings.Builder
	indent := 0

	prevWasNewline := false
	for si, seg := range segments {
		if !seg.isToken {
			if seg.text == " " && prevWasNewline {
				continue
			}
			if seg.text == "," {
				result.WriteString(",")
				prevWasNewline = false
				continue
			}
			result.WriteString(seg.text)
			prevWasNewline = false
			continue
		}

		upper := strings.ToUpper(seg.text)

		isJoinLine := false
		if joinKeywords[upper] {
			lookahead := ""
			for j := si + 1; j < len(segments); j++ {
				if segments[j].text == " " {
					continue
				}
				lookahead = strings.ToUpper(segments[j].text)
				break
			}
			if upper == "JOIN" || lookahead == "JOIN" || lookahead == "OUTER" {
				isJoinLine = true
			}
		}

		if upper == "ORDER" || upper == "GROUP" || upper == "LIMIT" || upper == "OFFSET" {
			result.WriteString("\n")
			result.WriteString(seg.text)
			prevWasNewline = false
			continue
		}

		if majorClauses[upper] && si > 0 {
			if upper == "SELECT" || upper == "INSERT" || upper == "UPDATE" || upper == "DELETE" {
				if si > 0 {
					result.WriteString("\n")
				}
			} else {
				result.WriteString("\n")
			}
			result.WriteString(seg.text)
			prevWasNewline = false
			if upper == "SELECT" || upper == "FROM" || upper == "WHERE" || upper == "SET" || upper == "HAVING" {
				indent = 1
			}
			continue
		}

		if isJoinLine {
			result.WriteString("\n")
			result.WriteString(seg.text)
			prevWasNewline = false
			continue
		}

		if indentClauses[upper] {
			result.WriteString("\n")
			result.WriteString(strings.Repeat("  ", indent))
			result.WriteString(seg.text)
			prevWasNewline = false
			continue
		}

		_ = indent
		result.WriteString(seg.text)
		prevWasNewline = false
	}

	formatted := result.String()
	var finalLines []string
	for _, line := range strings.Split(formatted, "\n") {
		trimmed := strings.TrimRight(line, " \t")
		finalLines = append(finalLines, trimmed)
	}

	return strings.TrimSpace(strings.Join(finalLines, "\n"))
}

func HighlightSQL(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return sql
	}

	keywordSet := make(map[string]bool)
	for _, kw := range sqlKeywords {
		keywordSet[kw] = true
	}

	functionSet := make(map[string]bool)
	for _, fn := range sqlFunctions {
		functionSet[fn] = true
	}

	commentRe := regexp.MustCompile(`--[^\n]*`)
	stringRe := regexp.MustCompile(`'(?:[^'\\]|\\.)*'`)
	numberRe := regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	operatorRe := regexp.MustCompile(`[=<>!]+|[+\-*/]`)

	type Token struct {
		Start int
		End   int
		Style lipgloss.Style
	}

	var tokens []Token

	for _, match := range commentRe.FindAllStringIndex(sql, -1) {
		tokens = append(tokens, Token{Start: match[0], End: match[1], Style: CommentStyle})
	}

	for _, match := range stringRe.FindAllStringIndex(sql, -1) {
		tokens = append(tokens, Token{Start: match[0], End: match[1], Style: StringStyle})
	}

	for _, match := range numberRe.FindAllStringIndex(sql, -1) {
		tokens = append(tokens, Token{Start: match[0], End: match[1], Style: NumberStyle})
	}

	for _, match := range operatorRe.FindAllStringIndex(sql, -1) {
		tokens = append(tokens, Token{Start: match[0], End: match[1], Style: OperatorStyle})
	}

	wordRe := regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*\b`)
	for _, match := range wordRe.FindAllStringIndex(sql, -1) {
		word := sql[match[0]:match[1]]
		upperWord := strings.ToUpper(word)
		if keywordSet[upperWord] {
			tokens = append(tokens, Token{Start: match[0], End: match[1], Style: KeywordStyle})
		} else if functionSet[upperWord] {
			tokens = append(tokens, Token{Start: match[0], End: match[1], Style: FunctionStyle})
		}
	}

	if len(tokens) == 0 {
		return sql
	}

	type Interval struct {
		Start int
		End   int
	}
	intervals := make([]Interval, len(tokens))
	for i, t := range tokens {
		intervals[i] = Interval{Start: t.Start, End: t.End}
	}

	overlapping := make([]bool, len(tokens))
	for i := 0; i < len(tokens); i++ {
		for j := i + 1; j < len(tokens); j++ {
			if intervals[i].Start < intervals[j].End && intervals[j].Start < intervals[i].End {
				if intervals[j].Start < intervals[i].Start ||
					(intervals[j].Start == intervals[i].Start && intervals[j].End > intervals[i].End) {
					overlapping[i] = true
				} else {
					overlapping[j] = true
				}
			}
		}
	}

	var filtered []Token
	for i, tok := range tokens {
		if !overlapping[i] {
			filtered = append(filtered, tok)
		}
	}

	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].Start > filtered[j].Start {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	var result strings.Builder
	pos := 0
	for _, tok := range filtered {
		if tok.Start > pos {
			result.WriteString(sql[pos:tok.Start])
		}
		result.WriteString(tok.Style.Render(sql[tok.Start:tok.End]))
		pos = tok.End
	}
	if pos < len(sql) {
		result.WriteString(sql[pos:])
	}

	return result.String()
}
