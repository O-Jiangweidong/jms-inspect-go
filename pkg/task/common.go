package task

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"fmt"
	"inspect/pkg/common"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/go-redis/redis"
	"github.com/liushuochen/gotable"
	"golang.org/x/crypto/ssh/terminal"
	"gopkg.in/yaml.v3"
)

const (
	CSV = 0
	YML = 1
)

type Command struct {
	content string
	timeout int
}

func (c *Command) Value() string {
	if c.timeout != 0 {
		return fmt.Sprintf("timeout %d %s", c.timeout, c.content)
	}
	return c.content
}

type GlobalInfo struct {
	Machines        []Machine
	JMSCount        int
	RDSCount        int
	RedisCount      int
	TotalCount      int
	InspectDatetime string
	JMSVersion      string
}

func IsValidType(machineType string) error {
	validTypes := []string{"mysql", "jumpserver"}
	for _, element := range validTypes {
		if element == machineType {
			return nil
		}
	}
	return fmt.Errorf("无效的类型 %s, 目前仅支持 %s", machineType, strings.Join(validTypes, ", "))
}

type ResultSummary struct {
	GlobalInfo GlobalInfo

	NormalResults   []map[string]interface{}
	AbnormalResults []AbnormalMsg
	VirtualResult   map[string]interface{}
	DBResult        map[string]interface{}

	// Other
	EchartsData string `json:"-"`
}

func (r *ResultSummary) SetGlobalInfo(opts *Options) {
	if version, exist := opts.JMSConfig["CURRENT_VERSION"]; exist {
		r.GlobalInfo.JMSVersion = version
	} else {
		r.GlobalInfo.JMSVersion = common.Empty
	}

	r.GlobalInfo.InspectDatetime = common.CurrentDatetime("time")
	r.GlobalInfo.Machines = opts.MachineSet
	for _, m := range opts.MachineSet {
		switch m.Type {
		case common.JumpServer:
			r.GlobalInfo.JMSCount += 1
		case common.MySQL:
		case common.PostgreSQL:
			r.GlobalInfo.RDSCount += 1
		case common.Redis:
			r.GlobalInfo.RedisCount += 1
		}
	}
	r.GlobalInfo.TotalCount = r.GlobalInfo.JMSCount + r.GlobalInfo.RedisCount + r.GlobalInfo.RDSCount
}

type Options struct {
	Logger *common.Logger

	// 命令行参数
	Debug           bool
	Silent          bool
	JMSConfigPath   string
	MachineInfoPath string
	ExcludeTask     string

	// 解析的参数
	JMSConfig    map[string]string
	MachineSet   []Machine
	RDSClient    RDSClient
	RedisClient  *redis.Client
	EnableRedis  bool
	EnableRDS    bool
	DebugLogFile *common.DebugLogger
}

func (o *Options) Clear() {
	if o.RDSClient != nil {
		_ = o.RDSClient.Close()
	}
	if o.RedisClient != nil {
		_ = o.RedisClient.Close()
	}
}

func (o *Options) PreDebug() error {
	if o.Debug == false {
		return nil
	}
	o.DebugLogFile = common.NewDebugLogger()
	common.AddCallback(func() {
		o.DebugLogFile.Close()
	})
	return nil
}

func (o *Options) Transform() {
	o.EnableRDS, o.EnableRedis = true, true
	for _, taskName := range strings.Split(o.ExcludeTask, ",") {
		switch strings.TrimSpace(taskName) {
		case "rds":
			o.EnableRDS = false
		case "redis":
			o.EnableRedis = false
		}
	}
}

func (o *Options) CheckJMSConfig() error {
	if _, err := os.Stat(o.JMSConfigPath); err != nil {
		return fmt.Errorf("请检查文件路径: %s，文件不存在。", o.JMSConfigPath)
	}

	if config, err := common.ConfigFileToMap(o.JMSConfigPath); err != nil {
		return fmt.Errorf("请检查文件路径: %s，解析文件失败。", o.JMSConfigPath)
	} else {
		o.JMSConfig = config
	}
	return nil
}

func (o *Options) FindCoreAuth() (string, string) {
	for _, m := range o.MachineSet {
		if m.Type == common.JumpServer && m.PriType != "" && m.PriPwd != "" {
			return m.PriType, m.PriPwd
		}
	}
	return "", ""
}

func (o *Options) GetHostFromDocker(host string) string {
	var finCommand string
	container := ""
	if host == "mysql" {
		container = "jms_mysql"
	} else if host == "redis" {
		container = "jms_redis"
	} else if host == "postgresql" {
		container = "jms_postgresql"
	}
	if container != "" {
		baseCommand := fmt.Sprintf("docker inspect -f '{{.NetworkSettings.Networks.jms_net.IPAddress}}' %s", container)
		if priType, priPwd := o.FindCoreAuth(); priType != "" && priPwd != "" {
			finCommand = fmt.Sprintf("echo %s | %s -c '%s' root", priPwd, priType, baseCommand)
		} else {
			finCommand = baseCommand
		}
		cmd := exec.Command("sh", "-c", finCommand)
		if ret, err := cmd.CombinedOutput(); err == nil {
			ipv4Regex := regexp.MustCompile(`([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3})`)
			matches := ipv4Regex.FindStringSubmatch(string(ret))
			if len(matches) > 1 {
				host = matches[1]
			}
		}
	}
	return host
}

func (o *Options) GetRDSClient() (RDSClient, error) {
	var dsn string
	host := o.GetHostFromDocker(o.JMSConfig["DB_HOST"])
	port := o.JMSConfig["DB_PORT"]
	database := o.JMSConfig["DB_NAME"]
	username := o.JMSConfig["DB_USER"]
	password := o.JMSConfig["DB_PASSWORD"]
	engine := o.JMSConfig["DB_ENGINE"]
	driverName := "mysql"
	if engine == common.PostgreSQL {
		driverName = "postgres"
		dsn = fmt.Sprintf(
			"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, username, password, database,
		)
	} else {
		dsn = fmt.Sprintf(
			"%s:%s@tcp(%s:%s)/%s",
			username, password, host, port, database,
		)
	}
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	return newSQLClient(engine, database, db), nil
}

func (o *Options) CheckRDS() error {
	if !o.EnableRDS {
		return nil
	}
	o.Logger.MsgOneLine(common.NoType, "根据 JC(JumpServer Config) 配置文件，检查 JumpServer 数据库是否可连接...")
	db, err := o.GetRDSClient()
	if err != nil {
		o.Logger.MsgOneLine(common.NoType, "")
		return err
	}
	o.RDSClient = db
	if err = db.Ping(); err != nil {
		o.Logger.MsgOneLine(common.NoType, "")
		return fmt.Errorf("连接 JumpServer RDS 失败: %v", err)
	}
	return nil
}

func (o *Options) GetSentinelRedisClient() *redis.Client {
	if sentinelHostString, exist := o.JMSConfig["REDIS_SENTINEL_HOSTS"]; exist {
		sentinelInfo := strings.SplitN(sentinelHostString, "/", 2)
		if len(sentinelInfo) != 2 {
			return nil
		}
		var err error
		var masterInfo map[string]string
		sentinelHosts := strings.Split(sentinelInfo[1], ",")
		for _, host := range sentinelHosts {
			sentinelClient := redis.NewSentinelClient(&redis.Options{
				Addr: host, Password: o.JMSConfig["REDIS_SENTINEL_PASSWORD"],
			})
			defer func(sentinelClient *redis.SentinelClient) {
				_ = sentinelClient.Close()
			}(sentinelClient)

			masterInfo, err = sentinelClient.Master(sentinelInfo[0]).Result()
			if err != nil {
				fmt.Printf("哨兵 %s 连接失败: %s\n", host, err)
			}
		}
		if _, exist = masterInfo["ip"]; !exist {
			return nil
		}
		return redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", masterInfo["ip"], masterInfo["port"]),
			Password: o.JMSConfig["REDIS_PASSWORD"],
		})
	}
	return nil
}

func (o *Options) GetSingleRedis(host, port, password string) *redis.Client {
	host = o.GetHostFromDocker(host)
	return redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: password,
	})
}

func (o *Options) GetRedisClient() *redis.Client {
	// 先检测是否使用哨兵
	if client := o.GetSentinelRedisClient(); client != nil {
		return client
	}
	// 再获取普通 Redis
	return o.GetSingleRedis(
		o.JMSConfig["REDIS_HOST"], o.JMSConfig["REDIS_PORT"], o.JMSConfig["REDIS_PASSWORD"],
	)
}

func (o *Options) CheckRedis() error {
	if !o.EnableRedis {
		return nil
	}
	o.Logger.MsgOneLine(common.NoType, "根据 JC(JumpServer Config) 配置文件，检查 JumpServer Redis 是否可连接...")
	rdb := o.GetRedisClient()
	o.RedisClient = rdb
	defer func(rdb *redis.Client) {
		_ = rdb.Close()
	}(rdb)
	if _, err := rdb.Ping().Result(); err != nil {
		o.Logger.MsgOneLine(common.NoType, "")
		return fmt.Errorf("连接 JumpServer Redis 失败: %v", err)
	}
	o.Logger.MsgOneLine(common.Success, "数据库连接测试成功\n\n")
	return nil
}

func (o *Options) CheckDB() error {
	if err := o.CheckRDS(); err != nil {
		return err
	}
	if err := o.CheckRedis(); err != nil {
		return err
	}
	return nil
}

func (o *Options) getPasswordFromUser(answer string) string {
	var password string
	for i := 1; i < 4; i++ {
		o.Logger.MsgOneLine(common.NoType, answer)
		if bytePassword, err := terminal.ReadPassword(int(syscall.Stdin)); err != nil {
			o.Logger.Error("输入有误!")
		} else {
			password = string(bytePassword)
			break
		}
	}
	return password
}

type configYML struct {
	Servers []Machine `yaml:"servers"`
}

func (o *Options) CheckMachine() error {
	if o.MachineInfoPath == "" {
		return fmt.Errorf("待巡检机器文件路径不能为空")
	}
	if _, err := os.Stat(o.MachineInfoPath); err != nil {
		return fmt.Errorf("请检查文件路径: %s，文件不存在", o.MachineInfoPath)
	}

	data, err := os.ReadFile(o.MachineInfoPath)
	if err != nil {
		return fmt.Errorf("请检查文件路径: %s，文件不存在", o.MachineInfoPath)
	}

	configType := CSV
	o.Logger.Debug("正在检查模板文件中机器是否有效...")
	reader := csv.NewReader(strings.NewReader(string(data)))
	rows, configErr := reader.ReadAll()
	if configErr != nil {
		configType = YML
	}

	var allMachines []Machine
	machineNameSet := make(map[string]bool)
	if configType == CSV {
		var nameIdx, typeIdx, hostIdx, portIdx, usernameIdx, passwordIdx int
		var privilegeTypeIdx, privilegePwdIdx = -1, -1
		for index, row := range rows {
			if index == 0 {
				for rowIdx, rowValue := range row {
					switch strings.ToLower(rowValue) {
					case "name":
						nameIdx = rowIdx
					case "type":
						typeIdx = rowIdx
					case "host":
						hostIdx = rowIdx
					case "port":
						portIdx = rowIdx
					case "username":
						usernameIdx = rowIdx
					case "password":
						passwordIdx = rowIdx
					case "privilege_type":
						privilegeTypeIdx = rowIdx
					case "privilege_password":
						privilegePwdIdx = rowIdx
					}
				}
				continue
			}
			if len(row) != 6 && len(row) != 8 {
				return fmt.Errorf("文件第 %v 行的机器配置内容不完整，请检查: %v", index+1, o.MachineInfoPath)
			}
			name, type_, host, port := row[nameIdx], row[typeIdx], row[hostIdx], row[portIdx]
			username, password := row[usernameIdx], row[passwordIdx]
			var privilegeType, privilegePwd = "", ""
			if privilegeTypeIdx != -1 {
				privilegeType = row[privilegeTypeIdx]
			}
			if privilegePwdIdx != -1 {
				privilegePwd = row[privilegePwdIdx]
			}
			machine := Machine{
				Name: name, Type: strings.ToLower(type_), Host: host, Port: port,
				Username: username, Password: password, PriType: privilegeType, PriPwd: privilegePwd,
			}
			allMachines = append(allMachines, machine)
		}
	} else {
		var config configYML
		ymlErr := yaml.Unmarshal(data, &config)
		if err != nil {
			msg := fmt.Sprintf("%s 或者 %s", configErr, ymlErr)
			return fmt.Errorf("读取机器模板文件 %s 失败: %s", o.MachineInfoPath, msg)
		}
		allMachines = append(allMachines, config.Servers...)
	}

	var invalidMachines []Machine
	tableTitle := []string{"名称", "类型", "主机地址", "主机端口", "主机用户名", "提权方式", "是否有效"}
	table, tableErr := gotable.Create(tableTitle...)
	if tableErr != nil {
		return fmt.Errorf("初始化表格显示器失败: [%v]", err)
	}
	for index, m := range allMachines {
		valid := "x"
		m.Type = strings.ToLower(m.Type)

		if err = IsValidType(m.Type); err != nil {
			return err
		}
		if m.Password == "" && m.SSHKeyPath == "" {
			o.Logger.MsgOneLine(common.NoType, "")
			title := fmt.Sprintf(
				"请输入主机为 %s(%v)，用户名 %s 的密码：",
				m.Name, m.Host, m.Username,
			)
			m.Password = o.getPasswordFromUser(title)
		}
		if m.PriType != "" && m.PriPwd == "" {
			title := fmt.Sprintf("请输入主机为 %s(%s)，root 的密码：", m.Name, m.Host)
			m.PriPwd = o.getPasswordFromUser(title)
		}
		if _, ok := machineNameSet[m.Name]; ok {
			return fmt.Errorf("待巡检机器名称重复，名称为: %s", m.Name)
		} else {
			machineNameSet[m.Name] = true
		}
		o.Logger.MsgOneLine(
			common.NoType, "\t%v: 正在检查机器 %s(%s) 是否可连接...",
			index+1, m.Name, m.Host,
		)
		if err = m.Connect(); err == nil {
			m.Valid = true
			o.MachineSet = append(o.MachineSet, m)
			valid = "✔"
		} else {
			m.Valid = false
			invalidMachines = append(invalidMachines, m)
		}
		priType := m.PriType
		if priType == "" {
			priType = common.EmptyFlag
		}
		_ = table.AddRow([]string{
			m.Name, m.Type, m.Host, m.Port, m.Username, priType, valid,
		})
	}

	o.Logger.MsgOneLine(common.NoType, "")
	if configType == CSV {
		var yamlData bytes.Buffer

		config := configYML{Servers: allMachines}
		encoder := yaml.NewEncoder(&yamlData)
		encoder.SetIndent(2)
		err = encoder.Encode(config)
		if err == nil {
			newPath := filepath.Join(filepath.Dir(o.MachineInfoPath), "auto-gen-config.yml")
			err = os.WriteFile(newPath, yamlData.Bytes(), 0644)
			if err == nil {
				o.Logger.Warning("CSV 格式配置文件自动转为 YML 格式，后续新配置均只在 YML 配置文件上支持，新配置文件路径: %v", newPath)
			}
		}
	}
	o.Logger.MsgOneLine(common.Success, "机器检查完成，具体如下：")
	if len(o.MachineSet) == 0 {
		fmt.Printf("\n%s\n", table)
		return fmt.Errorf("没有获取到有效的机器信息，请检查此文件内容: %s", o.MachineInfoPath)
	}
	if o.Silent {
		return nil
	}
	var answer string
	fmt.Printf("\n%s\n", table)
	fmt.Print("是否继续执行，本次任务只会执行有效资产(默认为 yes): ")
	_, _ = fmt.Scanln(&answer)
	answerStr := strings.ToLower(answer)
	if answerStr == "" || answerStr == "y" || answerStr == "yes" {
		return nil
	} else {
		os.Exit(0)
	}
	return nil
}

func (o *Options) Valid() error {
	if err := o.PreDebug(); err != nil {
		return err
	}
	o.Transform()
	if err := o.CheckJMSConfig(); err != nil {
		return err
	}
	if err := o.CheckMachine(); err != nil {
		return err
	}
	if err := o.CheckDB(); err != nil {
		return err
	}
	return nil
}
