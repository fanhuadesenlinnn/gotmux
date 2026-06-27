package command

import (
	"fmt"
	"strings"
	"unicode"
)

func ParseArgv(argv []string) ([][]string, error) {
	if len(argv) == 0 {
		return nil, nil
	}
	if len(argv) == 1 && strings.ContainsAny(argv[0], " \t\r\n;'\"\\") {
		return ParseString(argv[0])
	}
	var out [][]string
	var cur []string
	for _, arg := range argv {
		if arg == ";" {
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
			continue
		}
		parts, err := splitSemicolonArg(arg)
		if err != nil {
			return nil, err
		}
		for _, part := range parts {
			if part == ";" {
				if len(cur) > 0 {
					out = append(out, cur)
					cur = nil
				}
				continue
			}
			if part != "" {
				cur = append(cur, part)
			}
		}
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out, nil
}

func ParseScript(script string) ([][]string, error) {
	var out [][]string
	var logical strings.Builder
	for _, raw := range strings.Split(script, "\n") {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasSuffix(strings.TrimRight(line, " \t\r"), "\\") {
			line = strings.TrimRight(line, " \t\r")
			logical.WriteString(line[:len(line)-1])
			logical.WriteByte(' ')
			continue
		}
		logical.WriteString(line)
		commands, err := ParseString(logical.String())
		if err != nil {
			return nil, err
		}
		out = append(out, commands...)
		logical.Reset()
	}
	if strings.TrimSpace(logical.String()) != "" {
		commands, err := ParseString(logical.String())
		if err != nil {
			return nil, err
		}
		out = append(out, commands...)
	}
	return out, nil
}

func ParseString(s string) ([][]string, error) {
	var out [][]string
	var cur []string
	var tok strings.Builder
	var quote rune
	escaped := false
	emitToken := func() {
		if tok.Len() > 0 {
			cur = append(cur, tok.String())
			tok.Reset()
		}
	}
	emitCommand := func() {
		emitToken()
		if len(cur) > 0 {
			out = append(out, cur)
			cur = nil
		}
	}

	for _, r := range s {
		if escaped {
			tok.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
				continue
			}
			tok.WriteRune(r)
			continue
		}
		switch {
		case r == '\'' || r == '"':
			quote = r
		case r == ';':
			emitCommand()
		case unicode.IsSpace(r):
			emitToken()
		default:
			tok.WriteRune(r)
		}
	}
	if escaped {
		tok.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote")
	}
	emitCommand()
	return out, nil
}

func splitSemicolonArg(arg string) ([]string, error) {
	if !strings.Contains(arg, ";") {
		return []string{arg}, nil
	}
	var out []string
	var tok strings.Builder
	escaped := false
	for _, r := range arg {
		if escaped {
			tok.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == ';' {
			if tok.Len() > 0 {
				out = append(out, tok.String())
				tok.Reset()
			}
			out = append(out, ";")
			continue
		}
		tok.WriteRune(r)
	}
	if escaped {
		tok.WriteRune('\\')
	}
	if tok.Len() > 0 {
		out = append(out, tok.String())
	}
	return out, nil
}

func stripComment(s string) string {
	var out strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		if escaped {
			out.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			out.WriteRune(r)
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			}
			out.WriteRune(r)
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			out.WriteRune(r)
			continue
		}
		if r == '#' {
			break
		}
		out.WriteRune(r)
	}
	return out.String()
}
