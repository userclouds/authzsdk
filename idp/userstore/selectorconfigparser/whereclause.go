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

// NOTE: to update the parser after changes the lexer.nex and/or parser.y, do the following:
// 1) export GOPATH=/tmp/go
// 2) go install github.com/blynn/nex
// 3) go install golang.org/x/tools/cmd/goyacc@master
// 4) bin/nex -o idp/userstore/selectorconfigparser/lexer.go idp/userstore/selectorconfigparser.nex
// 5) bin/goyacc -o idp/userstore/selectorconfigparser/parser.go idp/userstore/selectorconfigparser/parser.y
// 6) revert changes to go.mod and go.sum from steps 2) and 3)
// 7) touch up generated .go files from 4) and 5) to satisfy lint rules
