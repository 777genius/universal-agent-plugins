package installerui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Small evidence screen for inline TUI cursor movement, with display-cell
// widths from the pinned ANSI parser. Raw PTY bytes remain available on failure.
// This is a text/control accessibility assertion, not a pixel golden.
type terminalTestScreen struct {
	rows, cols, row, col int
	grid                 [][]string
	pending              string
}

func newTerminalScreen(rows, cols int) *terminalTestScreen {
	s := &terminalTestScreen{rows: rows, cols: cols}
	for range rows {
		s.grid = append(s.grid, make([]string, cols))
	}
	return s
}
func (s *terminalTestScreen) resize(rows, cols int) {
	n := newTerminalScreen(rows, cols)
	for i := range min(rows, s.rows) {
		copy(n.grid[i], s.grid[i])
	}
	n.row, n.col = min(s.row, rows-1), min(s.col, cols-1)
	*s = *n
}
func (s *terminalTestScreen) feed(input string) {
	input = s.pending + input
	s.pending = ""
	for input != "" {
		seq, w, n, state := ansi.DecodeSequence(input, 0, nil)
		if n == 0 || state != 0 {
			s.pending = input
			return
		}
		input = input[n:]
		switch seq {
		case "\r":
			s.col = 0
		case "\n":
			s.row++
			if s.row >= s.rows {
				s.grid = append(s.grid[1:], make([]string, s.cols))
				s.row = s.rows - 1
			}
		case "\b":
			s.col = max(0, s.col-1)
		default:
			if strings.HasPrefix(seq, "\x1b[") {
				s.csi(seq)
				continue
			}
			if w <= 0 {
				continue
			}
			if s.col+w > s.cols {
				s.col = 0
				s.row = min(s.rows-1, s.row+1)
			}
			s.grid[s.row][s.col] = seq
			for i := 1; i < w; i++ {
				s.grid[s.row][s.col+i] = ""
			}
			s.col += w
		}
	}
}
func (s *terminalTestScreen) csi(seq string) {
	op := seq[len(seq)-1]
	params := strings.Split(seq[2:len(seq)-1], ";")
	value, _ := strconv.Atoi(params[0])
	n := max(1, value)
	switch op {
	case 'A':
		s.row = max(0, s.row-n)
	case 'B':
		s.row = min(s.rows-1, s.row+n)
	case 'C':
		s.col = min(s.cols-1, s.col+n)
	case 'D':
		s.col = max(0, s.col-n)
	case 'G':
		s.col = min(s.cols-1, n-1)
	case 'H', 'f':
		s.row = min(s.rows-1, n-1)
		s.col = 0
		if len(params) > 1 {
			col, _ := strconv.Atoi(params[1])
			s.col = min(s.cols-1, max(0, col-1))
		}
	case 'K':
		start, end := s.col, s.cols
		if value == 1 {
			start, end = 0, min(s.col+1, s.cols)
		}
		if value == 2 {
			start = 0
		}
		for i := start; i < end; i++ {
			s.grid[s.row][i] = ""
		}
	case 'J':
		if value == 2 || value == 3 {
			for i := range s.grid {
				clear(s.grid[i])
			}
		}
		if value == 0 {
			for i := s.row; i < s.rows; i++ {
				start := 0
				if i == s.row {
					start = min(s.col, s.cols)
				}
				clear(s.grid[i][start:])
			}
		}
	}
}
func (s *terminalTestScreen) text() string {
	var b strings.Builder
	for _, row := range s.grid {
		for _, cell := range row {
			if cell == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(cell)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
