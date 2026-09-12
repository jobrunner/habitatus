package rulepack

import (
	"fmt"
	"strings"
)

// ParseFormula parses a membership formula with R's operator precedence:
// NOT (unary "!" in R) binds tightest, then AND ("&"), then OR ("|").
// Upstream rewrites AND/OR/NOT to &/|/&! and hands the string to R's parser, so
// R's precedence is the behaviour we must reproduce. NOT is binary here ("and
// not"), not a prefix negation.
func ParseFormula(s string) (Node, error) {
	toks, err := tokenizeFormula(s)
	if err != nil {
		return nil, err
	}
	p := &formulaParser{toks: toks}
	n, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.toks) {
		return nil, fmt.Errorf("trailing input at token %d: %q", p.pos, p.toks[p.pos])
	}
	return n, nil
}

type formulaParser struct {
	toks []string
	pos  int
}

func (p *formulaParser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

// parseOr is the lowest precedence level.
func (p *formulaParser) parseOr() (Node, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.peek() == "OR" {
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = Or{L: left, R: right}
	}
	return left, nil
}

// parseAnd handles AND and NOT, both "&"-strength in R (NOT is "&!").
func (p *formulaParser) parseAnd() (Node, error) {
	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		switch p.peek() {
		case "AND":
			p.pos++
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = And{L: left, R: right}
		case "NOT":
			p.pos++
			right, err := p.parsePrimary()
			if err != nil {
				return nil, err
			}
			left = Not{L: left, R: right}
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parsePrimary() (Node, error) {
	switch t := p.peek(); {
	case t == "(":
		p.pos++
		n, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if p.peek() != ")" {
			return nil, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++
		return n, nil
	case strings.HasPrefix(t, "<"):
		p.pos++
		raw := strings.TrimSuffix(strings.TrimPrefix(t, "<"), ">")
		e, err := ParseExpr(raw)
		if err != nil {
			return nil, err
		}
		return Leaf{Expr: e, Raw: raw}, nil
	case t == "":
		return nil, fmt.Errorf("unexpected end of formula")
	default:
		return nil, fmt.Errorf("unexpected token %q", t)
	}
}

// tokenizeFormula splits into "(", ")", "AND", "OR", "NOT" and "<...>" chunks.
func tokenizeFormula(s string) ([]string, error) {
	var out []string
	i := 0
	for i < len(s) {
		switch c := s[i]; {
		case c == ' ' || c == '\t':
			i++
		case c == '(' || c == ')':
			out = append(out, string(c))
			i++
		case c == '<':
			j := strings.IndexByte(s[i:], '>')
			if j < 0 {
				return nil, fmt.Errorf("unterminated expression at offset %d", i)
			}
			out = append(out, s[i:i+j+1])
			i += j + 1
		default:
			j := i
			for j < len(s) && s[j] != ' ' && s[j] != '(' && s[j] != ')' && s[j] != '<' {
				j++
			}
			word := s[i:j]
			switch word {
			case "AND", "OR", "NOT":
				out = append(out, word)
			default:
				return nil, fmt.Errorf("unexpected word %q at offset %d", word, i)
			}
			i = j
		}
	}
	return out, nil
}
