package common

import (
	"fmt"
	"os"
)

type Logger struct {
}

func (l *Logger) Format(prefix, format string, a ...any) {
	content := fmt.Sprintf(format, a...)
	fmt.Printf("[%s]:> %s\n", prefix, content)
}

func (l *Logger) MsgOneLine(format string, a ...any) {
	fmt.Printf("%s\r", fmt.Sprintf(format, a...))
}

func (l *Logger) Debug(format string, a ...any) {
	l.Format("DEBUG", format, a...)
}

func (l *Logger) Info(format string, a ...any) {
	l.Format("INFO", format, a...)
}

func (l *Logger) Warning(format string, a ...any) {
	l.Format("WARNING", format, a...)
}

func (l *Logger) Error(format string, a ...any) {
	l.Format("ERROR", format, a...)
	os.Exit(1)
}

func NewLogger() *Logger {
	return &Logger{}
}
