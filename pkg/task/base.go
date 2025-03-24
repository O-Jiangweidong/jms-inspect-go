package task

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"inspect/pkg/common"
)

type Machine struct {
	Name       string `yaml:"name" json:"name"`
	Type       string `yaml:"type" json:"type"`
	Host       string `yaml:"host" json:"host"`
	Port       string `yaml:"port" json:"port"`
	Username   string `yaml:"username" json:"username"`
	Password   string `yaml:"password" json:"-"`
	SSHKeyPath string `yaml:"ssh_key_path" json:"-"`
	PriType    string `yaml:"privilege_type" json:"privilege_type"`
	PriPwd     string `yaml:"privilege_password" json:"-"`
	Valid      bool   `yaml:"valid" json:"valid"`

	Client *ssh.Client `yaml:"-" json:"-"`
}

func (m *Machine) Connect() error {
	auth := []ssh.AuthMethod{
		ssh.Password(m.Password),
	}
	if m.SSHKeyPath != "" {
		key, err := os.ReadFile(m.SSHKeyPath)
		if err != nil {
			return fmt.Errorf("密钥文件读取失败: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return fmt.Errorf("密钥文件解析失败: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	}
	sshConfig := &ssh.ClientConfig{
		User:            m.Username,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	address := fmt.Sprintf("%s:%s", m.Host, m.Port)
	if client, err := ssh.Dial("tcp", address, sshConfig); err != nil {
		return err
	} else {
		m.Client = client
		command := Command{content: "whoami", timeout: 5}
		if _, err = m.DoCommand(command); err != nil {
			return err
		}
		return nil
	}
}

func (m *Machine) DoCommand(command Command) (string, error) {
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
		cmd := strings.ReplaceAll(command.Value(), "'", "'\\''")
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
		rest, err = session.CombinedOutput(command.Value())
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
