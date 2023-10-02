package selectorconfigparser

import (
	"strings"

	"userclouds.com/infra/ucerr"
)

type customLexer struct {
	*lexer
	ErrorOutput string
}

func (l *customLexer) Error(s string) {
	l.ErrorOutput = s
}

// ParseWhereClause parses a where clause and returns an error if it is invalid
func ParseWhereClause(clause string) error {
	input := strings.NewReader(clause)
	cl := &customLexer{newLexer(input), ""}
	if yyParse(cl) != 0 {
		return ucerr.Friendlyf(nil, "error parsing where clause: %s", cl.ErrorOutput)
	}
	return nil
}

// "nex -o lexer.go lexer.nex && goyacc -o parser.go parser.y" used to generate lexer.go and parser.go (and then touched up for lint)
