package selectorconfigparser

import (
	"bufio"
	"io"
	"strings"
)

type frame struct {
	i            int
	s            string
	line, column int
}

type lexer struct {
	// The lexer runs in its own goroutine, and communicates via channel 'ch'.
	ch     chan frame
	chStop chan bool
	// We record the level of nesting because the action could return, and a
	// subsequent call expects to pick up where it left off. In other words,
	// we're simulating a coroutine.
	// TODO: Support a channel-based variant that compatible with Go's yacc.
	stack []frame
	stale bool
}

// NewLexerWithInit creates a new lexer object, runs the given callback on it,
// then returns it.
func newLexerWithInit(in io.Reader, initFun func(*lexer)) *lexer {
	yyLex := new(lexer)
	if initFun != nil {
		initFun(yyLex)
	}
	yyLex.ch = make(chan frame)
	yyLex.chStop = make(chan bool, 1)
	var scan func(in *bufio.Reader, ch chan frame, chStop chan bool, family []dfa, line, column int)
	scan = func(in *bufio.Reader, ch chan frame, chStop chan bool, family []dfa, line, column int) {
		// Index of DFA and length of highest-precedence match so far.
		matchi, matchn := 0, -1
		var buf []rune
		n := 0
		checkAccept := func(i int, st int) bool {
			// Higher precedence match? DFAs are run in parallel, so matchn is at most len(buf), hence we may omit the length equality check.
			if family[i].acc[st] && (matchn < n || matchi > i) {
				matchi, matchn = i, n
				return true
			}
			return false
		}
		var state [][2]int
		for i := 0; i < len(family); i++ {
			mark := make([]bool, len(family[i].startf))
			// Every DFA starts at state 0.
			st := 0
			for {
				state = append(state, [2]int{i, st})
				mark[st] = true
				// As we're at the start of input, follow all ^ transitions and append to our list of start states.
				st = family[i].startf[st]
				if st == -1 || mark[st] {
					break
				}
				// We only check for a match after at least one transition.
				checkAccept(i, st)
			}
		}
		atEOF := false
		stopped := false
		for {
			if n == len(buf) && !atEOF {
				r, _, err := in.ReadRune()
				switch err {
				case io.EOF:
					atEOF = true
				case nil:
					buf = append(buf, r)
				default:
					panic(err)
				}
			}
			if !atEOF {
				r := buf[n]
				n++
				var nextState [][2]int
				for _, x := range state {
					x[1] = family[x[0]].f[x[1]](r)
					if x[1] == -1 {
						continue
					}
					nextState = append(nextState, x)
					checkAccept(x[0], x[1])
				}
				state = nextState
			} else {
			dollar: // Handle $.
				for _, x := range state {
					mark := make([]bool, len(family[x[0]].endf))
					for {
						mark[x[1]] = true
						x[1] = family[x[0]].endf[x[1]]
						if x[1] == -1 || mark[x[1]] {
							break
						}
						if checkAccept(x[0], x[1]) {
							// Unlike before, we can break off the search. Now that we're at the end, there's no need to maintain the state of each DFA.
							break dollar
						}
					}
				}
				state = nil
			}

			if state == nil {
				lcUpdate := func(r rune) {
					if r == '\n' {
						line++
						column = 0
					} else {
						column++
					}
				}
				// All DFAs stuck. Return last match if it exists, otherwise advance by one rune and restart all DFAs.
				if matchn == -1 {
					if len(buf) == 0 { // This can only happen at the end of input.
						break
					}
					lcUpdate(buf[0])
					buf = buf[1:]
				} else {
					text := string(buf[:matchn])
					buf = buf[matchn:]
					matchn = -1
					select {
					case ch <- frame{matchi, text, line, column}:
						{
						}
					case stopped = <-chStop:
						{
						}
					}
					if stopped {
						break
					}
					if len(family[matchi].nest) > 0 {
						scan(bufio.NewReader(strings.NewReader(text)), ch, chStop, family[matchi].nest, line, column)
					}
					if atEOF {
						break
					}
					for _, r := range text {
						lcUpdate(r)
					}
				}
				n = 0
				for i := 0; i < len(family); i++ {
					state = append(state, [2]int{i, 0})
				}
			}
		}
		ch <- frame{-1, "", line, column}
	}
	go scan(bufio.NewReader(in), yyLex.ch, yyLex.chStop, dfas, 0, 0)
	return yyLex
}

type dfa struct {
	acc          []bool           // Accepting states.
	f            []func(rune) int // Transitions.
	startf, endf []int            // Transitions at start and end of input.
	nest         []dfa
}

var dfas = []dfa{
	// {[a-zA-Z0-9_-]+}(->>'[a-zA-Z0-9_-]+')?
	{[]bool{false, false, false, true, false, false, false, false, false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return -1
			case 62:
				return -1
			case 95:
				return -1
			case 123:
				return 1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return 2
			case 62:
				return -1
			case 95:
				return 2
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return 2
			case 65 <= r && r <= 90:
				return 2
			case 97 <= r && r <= 122:
				return 2
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return 2
			case 62:
				return -1
			case 95:
				return 2
			case 123:
				return -1
			case 125:
				return 3
			}
			switch {
			case 48 <= r && r <= 57:
				return 2
			case 65 <= r && r <= 90:
				return 2
			case 97 <= r && r <= 122:
				return 2
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return 4
			case 62:
				return -1
			case 95:
				return -1
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return -1
			case 62:
				return 5
			case 95:
				return -1
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return -1
			case 62:
				return 6
			case 95:
				return -1
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return 7
			case 45:
				return -1
			case 62:
				return -1
			case 95:
				return -1
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return 8
			case 62:
				return -1
			case 95:
				return 8
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return 8
			case 65 <= r && r <= 90:
				return 8
			case 97 <= r && r <= 122:
				return 8
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return 9
			case 45:
				return 8
			case 62:
				return -1
			case 95:
				return 8
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return 8
			case 65 <= r && r <= 90:
				return 8
			case 97 <= r && r <= 122:
				return 8
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 39:
				return -1
			case 45:
				return -1
			case 62:
				return -1
			case 95:
				return -1
			case 123:
				return -1
			case 125:
				return -1
			}
			switch {
			case 48 <= r && r <= 57:
				return -1
			case 65 <= r && r <= 90:
				return -1
			case 97 <= r && r <= 122:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, []int{ /* End-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, nil},

	// =|<=|>=|<|>|!=| LIKE | ILIKE | like | ilike
	{[]bool{false, false, false, true, true, true, true, true, true, false, false, false, false, false, false, false, true, false, false, false, false, true, false, false, false, true, false, false, false, false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 32:
				return 1
			case 33:
				return 2
			case 60:
				return 3
			case 61:
				return 4
			case 62:
				return 5
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return 9
			case 75:
				return -1
			case 76:
				return 10
			case 101:
				return -1
			case 105:
				return 11
			case 107:
				return -1
			case 108:
				return 12
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return 8
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return 7
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return 6
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return 26
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return 22
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return 17
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return 13
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return 14
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return 15
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 16
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return 18
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return 19
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return 20
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 21
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return 23
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return 24
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 25
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return 27
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return 28
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return 29
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 30
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 33:
				return -1
			case 60:
				return -1
			case 61:
				return -1
			case 62:
				return -1
			case 69:
				return -1
			case 73:
				return -1
			case 75:
				return -1
			case 76:
				return -1
			case 101:
				return -1
			case 105:
				return -1
			case 107:
				return -1
			case 108:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, []int{ /* End-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, nil},

	// \?
	{[]bool{false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 63:
				return 1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 63:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1}, []int{ /* End-of-input transitions */ -1, -1}, nil},

	// ANY|any
	{[]bool{false, false, false, false, true, false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 65:
				return 1
			case 78:
				return -1
			case 89:
				return -1
			case 97:
				return 2
			case 110:
				return -1
			case 121:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return 5
			case 89:
				return -1
			case 97:
				return -1
			case 110:
				return -1
			case 121:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return -1
			case 89:
				return -1
			case 97:
				return -1
			case 110:
				return 3
			case 121:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return -1
			case 89:
				return -1
			case 97:
				return -1
			case 110:
				return -1
			case 121:
				return 4
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return -1
			case 89:
				return -1
			case 97:
				return -1
			case 110:
				return -1
			case 121:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return -1
			case 89:
				return 6
			case 97:
				return -1
			case 110:
				return -1
			case 121:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 65:
				return -1
			case 78:
				return -1
			case 89:
				return -1
			case 97:
				return -1
			case 110:
				return -1
			case 121:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1, -1, -1, -1, -1, -1}, []int{ /* End-of-input transitions */ -1, -1, -1, -1, -1, -1, -1}, nil},

	// AND | OR | and | or
	{[]bool{false, false, false, false, false, false, false, true, false, false, true, false, true, false, false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 32:
				return 1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return 2
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return 3
			case 82:
				return -1
			case 97:
				return 4
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return 5
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return 13
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return 11
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return 8
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return 6
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 7
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return 9
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 10
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 12
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return 14
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return 15
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 32:
				return -1
			case 65:
				return -1
			case 68:
				return -1
			case 78:
				return -1
			case 79:
				return -1
			case 82:
				return -1
			case 97:
				return -1
			case 100:
				return -1
			case 110:
				return -1
			case 111:
				return -1
			case 114:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, []int{ /* End-of-input transitions */ -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, nil},

	// \(
	{[]bool{false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 40:
				return 1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 40:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1}, []int{ /* End-of-input transitions */ -1, -1}, nil},

	// \)
	{[]bool{false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 41:
				return 1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 41:
				return -1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1}, []int{ /* End-of-input transitions */ -1, -1}, nil},

	// [ \t\n]+
	{[]bool{false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			switch r {
			case 9:
				return 1
			case 10:
				return 1
			case 32:
				return 1
			}
			return -1
		},
		func(r rune) int {
			switch r {
			case 9:
				return 1
			case 10:
				return 1
			case 32:
				return 1
			}
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1}, []int{ /* End-of-input transitions */ -1, -1}, nil},

	// .
	{[]bool{false, true}, []func(rune) int{ // Transitions
		func(r rune) int {
			return 1
		},
		func(r rune) int {
			return -1
		},
	}, []int{ /* Start-of-input transitions */ -1, -1}, []int{ /* End-of-input transitions */ -1, -1}, nil},
}

func newLexer(in io.Reader) *lexer {
	return newLexerWithInit(in, nil)
}

func (yyLex *lexer) Stop() {
	yyLex.chStop <- true
}

// Text returns the matched text.
func (yyLex *lexer) Text() string {
	return yyLex.stack[len(yyLex.stack)-1].s
}

// Line returns the current line number.
// The first line is 0.
func (yyLex *lexer) Line() int {
	if len(yyLex.stack) == 0 {
		return 0
	}
	return yyLex.stack[len(yyLex.stack)-1].line
}

// Column returns the current column number.
// The first column is 0.
func (yyLex *lexer) Column() int {
	if len(yyLex.stack) == 0 {
		return 0
	}
	return yyLex.stack[len(yyLex.stack)-1].column
}

func (yyLex *lexer) next(lvl int) int {
	if lvl == len(yyLex.stack) {
		l, c := 0, 0
		if lvl > 0 {
			l, c = yyLex.stack[lvl-1].line, yyLex.stack[lvl-1].column
		}
		yyLex.stack = append(yyLex.stack, frame{0, "", l, c})
	}
	if lvl == len(yyLex.stack)-1 {
		p := &yyLex.stack[lvl]
		*p = <-yyLex.ch
		yyLex.stale = false
	} else {
		yyLex.stale = true
	}
	return yyLex.stack[lvl].i
}
func (yyLex *lexer) pop() {
	yyLex.stack = yyLex.stack[:len(yyLex.stack)-1]
}
func (yyLex lexer) Error(e string) {
	panic(e)
}

// Lex runs the lexer. Always returns 0.
// When the -s option is given, this function is not generated;
// instead, the NN_FUN macro runs the lexer.
func (yyLex *lexer) Lex(lval *yySymType) int {
OUTER0:
	for {
		switch yyLex.next(0) {
		case 0:
			{
				return COLUMN_IDENTIFIER
			}
		case 1:
			{
				return OPERATOR
			}
		case 2:
			{
				return VALUE_PLACEHOLDER
			}
		case 3:
			{
				return ANY
			}
		case 4:
			{
				return CONJUNCTION
			}
		case 5:
			{
				return LEFT_PARENTHESIS
			}
		case 6:
			{
				return RIGHT_PARENTHESIS
			}
		case 7:
			{ /* eat up whitespace */
			}
		case 8:
			{
				return UNKNOWN
			}
		default:
			break OUTER0
		}
		continue
	}
	yyLex.pop()

	return 0
}
