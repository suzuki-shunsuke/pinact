package run

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

type colorFunc func(a ...interface{}) string

type Logger struct {
	stderr io.Writer
	red    colorFunc
	green  colorFunc
}

func NewLogger(stderr io.Writer) *Logger {
	return &Logger{
		red:    color.New(color.FgRed).SprintFunc(),
		green:  color.New(color.FgGreen).SprintFunc(),
		stderr: stderr,
	}
}

const levelError = "error"

func (l *Logger) Output(level, message string, line *Line, newLine string) {
	s := "INFO"
	if level == levelError {
		s = l.red("ERROR")
	}
	if newLine == "" {
		fmt.Fprintf(l.stderr, `%s %s
%s:%d
%s
`, s, message, line.File, line.Number, line.Line)
		return
	}
	fmt.Fprintf(l.stderr, `%s %s
%s:%d
%s
%s
`, s, message, line.File, line.Number, l.red("- "+line.Line), l.green("+ "+newLine))
}
