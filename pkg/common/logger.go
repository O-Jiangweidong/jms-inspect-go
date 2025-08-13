package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

const (
	Debug = iota
	Info
	Warning
	Success
	Error
	Period
	StopMsg
	NoType
)

const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
)

var (
	once   sync.Once
	logger *Logger
)

type LogMsg struct {
	Content  string
	Type     uint
	IsFather bool
}

type Logger struct {
	spinnerFlag bool
	silent      bool
	stopChan    chan bool // 退出信号通道
	msgChan     chan *LogMsg
	msgCache    []string
	sync.Mutex  // 锁
}

func (l *Logger) SetSilent() {
	l.silent = true
}

func (l *Logger) logPrintForever() {
	for {
		select {
		case msg := <-l.msgChan:
			if msg.Type == Period || msg.Type == StopMsg {
				fmt.Print(msg.Content)
			} else if msg.Type == Period && !msg.IsFather {
				l.msgCache = append(l.msgCache, msg.Content)
			} else {
				for _, oldMsg := range l.msgCache {
					fmt.Print(oldMsg)
				}
				l.msgCache = []string{}
				fmt.Print(msg.Content)
			}
			if msg.Type == Error {
				l.Exit(1)
			}
		}
	}
}

func (l *Logger) pushSpinnerMsg(msg string) {
	spinChars := `-\|`
	i := 0
	for {
		select {
		case <-l.stopChan:
			return
		default:
			content := fmt.Sprintf("\r[%c]:> %s", spinChars[i%len(spinChars)], msg)
			logMsg := l.format(Period, false, content)
			logMsg.IsFather = i == 0
			l.PushMsg(logMsg)
			time.Sleep(200 * time.Millisecond) // 减少sleep的时间，提高响应速度
			i++
		}
	}
}

func (l *Logger) format(mType uint, newLine bool, format string, a ...any) *LogMsg {
	var prefix, content string
	var colorPre, colorSuf string
	switch mType {
	case Debug:
		prefix = "提示"
	case Info:
		prefix = "信息"
	case Warning:
		prefix, colorPre, colorSuf = "警告", Yellow, Reset
	case Error:
		prefix, colorPre, colorSuf = "错误", Red, Reset
	case Success:
		prefix, colorPre, colorSuf = "成功", Green, Reset
	default:
		prefix, colorPre, colorSuf = "", "", ""
	}
	if prefix != "" {
		content = fmt.Sprintf("[%s]:> %s", prefix, fmt.Sprintf(format, a...))
	} else {
		content = fmt.Sprintf(format, a...)
	}
	content = fmt.Sprintf("%s%s%s", colorPre, content, colorSuf)
	if newLine {
		content = fmt.Sprintf("%s%s", content, "\n")
	}
	return &LogMsg{Type: mType, Content: content}
}

func (l *Logger) PushMsg(logMsg *LogMsg) {
	if l.silent && logMsg.Type != Error {
		return
	}
	if l.msgChan != nil {
		l.msgChan <- logMsg
	}
}

func (l *Logger) MsgOneLine(mType uint, format string, a ...any) {
	width, _ := GetTerminalWidth()
	l.PushMsg(l.format(mType, false, "\r%s", strings.Repeat(" ", width)))
	logMsg := l.format(mType, false, fmt.Sprintf(format, a...))
	logMsg.Content = "\r" + logMsg.Content
	l.PushMsg(logMsg)
}

func (l *Logger) StartTip(format string, a ...any) {
	l.spinnerFlag = true
	go l.pushSpinnerMsg(fmt.Sprintf(format, a...))
}

func (l *Logger) StopTip(format string, a ...any) {
	l.spinnerFlag = false
	l.stopChan <- false
	width, _ := GetTerminalWidth()
	l.PushMsg(l.format(StopMsg, false, "\r%s", strings.Repeat(" ", width)))
	l.PushMsg(l.format(StopMsg, false, "\r%s\n", fmt.Sprintf(format, a...)))
}

func (l *Logger) Debug(format string, a ...any) {
	l.PushMsg(l.format(Debug, true, format, a...))
}

func (l *Logger) Info(format string, a ...any) {
	l.PushMsg(l.format(Info, true, format, a...))
}

func (l *Logger) Warning(format string, a ...any) {
	l.PushMsg(l.format(Warning, true, format, a...))
}

func (l *Logger) Error(format string, a ...any) {
	l.PushMsg(l.format(Error, true, format, a...))
}

func (l *Logger) Finished(format string, a ...any) {
	fmt.Println(fmt.Sprintf(format, a...))
	l.Exit(0)
}

func (l *Logger) Exit(code int) {
	for _, callback := range FinishedCallbacks {
		callback()
	}
	os.Exit(code)
}

func newLogger() *Logger {
	logger := &Logger{
		msgChan:     make(chan *LogMsg),
		stopChan:    make(chan bool),
		spinnerFlag: false,
	}
	go logger.logPrintForever()
	return logger
}

func GetLogger() *Logger {
	once.Do(func() {
		logger = newLogger()
	})
	return logger
}

type DebugLogger struct {
	file    *os.File
	content string
}

func (l *DebugLogger) Write(obj interface{}) error {
	jsonData, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return err
	}
	if l.content != "" {
		l.content += "\n"
	}
	l.content += string(jsonData)
	return nil
}

func (l *DebugLogger) Close() {
	if l.file != nil {
		_, _ = l.file.WriteString(l.content)
		_ = l.file.Close()
	}
}

func NewDebugLogger() *DebugLogger {
	filePath := path.Join(OutputDir, "inspect.log")
	file, _ := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	return &DebugLogger{file: file, content: ""}
}
