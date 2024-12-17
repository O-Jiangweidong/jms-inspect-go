package task

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"inspect/pkg/common"
)

type Machine struct {
	Name     string
	Type     string
	Host     string
	Port     string
	Username string
	Password string `json:"-"`
	PriType  string
	PriPwd   string `json:"-"`
	Valid    bool

	Client *ssh.Client `json:"-"`
}

func (m *Machine) Connect() error {
	sshConfig := &ssh.ClientConfig{
		User:            m.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(m.Password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	address := fmt.Sprintf("%s:%s", m.Host, m.Port)
	if client, err := ssh.Dial("tcp", address, sshConfig); err != nil {
		return err
	} else {
		m.Client = client
		if _, err = m.DoCommand("whoami"); err != nil {
			return err
		}
		return nil
	}
}

func (m *Machine) DoCommand(cmd string) (string, error) {
	cmd = fmt.Sprintf("timeout 5s %s", cmd)
	session, err := m.Client.NewSession()
	if err != nil {
		return "", err
	}
	defer func(session *ssh.Session) {
		_ = session.Close()
	}(session)

	var rest []byte
	if strings.HasPrefix(m.PriType, "su") {
		var stdoutBuf bytes.Buffer
		session.Stdout = &stdoutBuf
		stdin, err := session.StdinPipe()
		if err != nil {
			return "", err
		}
		cmd = strings.ReplaceAll(cmd, "'", "'\\''")
		if err = session.Start(fmt.Sprintf("%s -c '%s'", m.PriType, cmd)); err != nil {
			return "", err
		}

		go func() {
			_, _ = stdin.Write([]byte(m.PriPwd + "\n"))
			_ = stdin.Close()
		}()

		if err = session.Wait(); err != nil {
			return "", err
		}
		rest = stdoutBuf.Bytes()
	} else {
		rest, err = session.CombinedOutput(cmd)
		if err != nil {
			return "", err
		}
	}
	return strings.TrimSpace(string(rest)), nil
}

func (m *Machine) Down() {
	if m.Client != nil {
		_ = m.Client.Close()
	}
}

func (m *Machine) GetExecutor() *Executor {
	executor := Executor{Machine: m}
	executor.Tasks = m.GetTasks()
	return &executor
}

func (m *Machine) GetTasks() []AbstractTask {
	generalTasks := []AbstractTask{&OsInfoTask{Machine: m}}
	switch m.Type {
	case common.JumpServer:
		generalTasks = append(generalTasks, &ServiceTask{Machine: m})
	}
	return generalTasks
}

type AbnormalMsg struct {
	Level        string
	Desc         string
	NodeName     string
	LevelDisplay string
}

type AbstractTask interface {
	Init(options *Options) error
	GetName() string
	Run() error
	GetResult() (map[string]interface{}, []AbnormalMsg)
}

type Task struct {
	result         map[string]interface{}
	abnormalResult []AbnormalMsg

	Machine   *Machine
	Options   *Options
	JMSConfig map[string]string
}

type Executor struct {
	Machine *Machine
	Tasks   []AbstractTask

	Result         map[string]interface{}
	AbnormalResult []AbnormalMsg
	Logger         *common.Logger
}

func (e *Executor) Execute(opts *Options) (map[string]interface{}, []AbnormalMsg) {
	e.Logger.Info("开始执行机器名为 [%s] 的任务，共%v个", e.Machine.Name, len(e.Tasks))
	e.Result = make(map[string]interface{})
	for _, t := range e.Tasks {
		e.MergeResult(DoTask(t, opts))
	}
	e.Machine.Down()
	e.Logger.Info("机器名为 [%s] 的任务全部执行结束\n", e.Machine.Name)
	return e.Result, e.AbnormalResult
}

func (e *Executor) MergeResult(result map[string]interface{}, abnormalResult []AbnormalMsg) {
	for key, value := range result {
		e.Result[key] = value
	}
	e.AbnormalResult = append(e.AbnormalResult, abnormalResult...)
}

func (t *Task) Init(opts *Options) error {
	t.Options = opts
	t.result = make(map[string]interface{})
	return nil
}

func (t *Task) GetConfig(key, input string) string {
	if v, exist := t.Options.JMSConfig[key]; exist {
		return v
	} else {
		return input
	}
}

func (t *Task) SetAbnormalEvent(desc, level string) {
	displayMap := make(map[string]string)
	displayMap[common.Critical] = "严重"
	displayMap[common.Normal] = "一般"
	displayMap[common.Slight] = "轻微"

	t.abnormalResult = append(t.abnormalResult, AbnormalMsg{
		Level: level, Desc: desc, LevelDisplay: displayMap[level],
	})
}

func (t *Task) GetResult() (map[string]interface{}, []AbnormalMsg) {
	return t.result, t.abnormalResult
}
