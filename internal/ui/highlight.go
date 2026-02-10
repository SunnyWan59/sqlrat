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
