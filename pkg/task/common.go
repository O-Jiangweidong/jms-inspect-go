package task

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"github.com/go-redis/redis"
	_ "github.com/go-sql-driver/mysql"
	"github.com/liushuochen/gotable"
	"golang.org/x/crypto/ssh/terminal"
	"inspect/pkg/common"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

type GlobalInfo struct {
	Machines        []Machine
	JMSCount        int
	MySQLCount      int
	RedisCount      int
	TotalCount      int
	InspectDatetime string
	JMSVersion      string
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
			r.GlobalInfo.MySQLCount += 1
		case common.Redis:
			r.GlobalInfo.RedisCount += 1
		}
	}
	r.GlobalInfo.TotalCount = r.GlobalInfo.JMSCount + r.GlobalInfo.RedisCount + r.GlobalInfo.MySQLCount
}

type Options struct {
	Logger *common.Logger

	// 命令行参数
	Debug           bool
	JMSConfigPath   string
	MachineInfoPath string
	ExcludeTask     string

	// 解析的参数
	JMSConfig   map[string]string
	MachineSet  []Machine
	MySQLClient *sql.DB
	RedisClient *redis.Client
	EnableRedis bool
	EnableMySQL bool
}

func (o *Options) Clear() {
	if o.MySQLClient != nil {
		_ = o.MySQLClient.Close()
	}
	if o.RedisClient != nil {
		_ = o.RedisClient.Close()
	}
}

func (o *Options) Transform() {
	o.EnableMySQL, o.EnableRedis = true, true
	for _, taskName := range strings.Split(o.ExcludeTask, ",") {
		switch strings.TrimSpace(taskName) {
		case "mysql":
			o.EnableMySQL = false
		case "redis":
			o.EnableRedis = false
		}
	}
}

func (o *Options) CheckJMSConfig() error {
	if _, err := os.Stat(o.JMSConfigPath); err != nil {
		return fmt.Errorf("请检查文件路径: [%s]，文件不存在。", o.JMSConfigPath)
	}

	if config, err := common.ConfigFileToMap(o.JMSConfigPath); err != nil {
		return fmt.Errorf("请检查文件路径: [%s]，解析文件失败。", o.JMSConfigPath)
	} else {
		o.JMSConfig = config
	}
	return nil
}

func (o *Options) GetMySQLClient() (*sql.DB, error) {
	host := o.JMSConfig["DB_HOST"]
	if host == "mysql" {
		cmd := exec.Command("docker", "inspect", "-f", "'{{.NetworkSettings.Networks.jms_net.IPAddress}}'", "jms_mysql")
		if ret, err := cmd.CombinedOutput(); err == nil {
			host = strings.Replace(string(ret), "'", "", -1)
			host = strings.TrimSpace(host)
		}
	}
	port := o.JMSConfig["DB_PORT"]
	database := o.JMSConfig["DB_NAME"]
	username := o.JMSConfig["DB_USER"]
	password := o.JMSConfig["DB_PASSWORD"]
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", username, password, host, port, database)
	return sql.Open("mysql", dsn)
}

func (o *Options) CheckMySQL() error {
	if !o.EnableMySQL {
		return nil
	}
	o.Logger.MsgOneLine(common.NoType, "根据 JC(JumpServer config) 配置文件，检查 JumpServer MySQL 是否可连接...")
	db, err := o.GetMySQLClient()
	if err != nil {
		o.Logger.MsgOneLine(common.NoType, "")
		return err
	}
	o.MySQLClient = db
	if err = db.Ping(); err != nil {
		o.Logger.MsgOneLine(common.NoType, "")
		return fmt.Errorf("连接 JumpServer MySQL 失败: %v", err)
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
				fmt.Printf("哨兵[%s]连接失败: %s\n", host, err)
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
	if host == "redis" {
		cmd := exec.Command(
			"docker", "inspect", "-f",
			"'{{.NetworkSettings.Networks.jms_net.IPAddress}}'", "jms_redis",
		)
		if ret, err := cmd.CombinedOutput(); err == nil {
			host = strings.Replace(string(ret), "'", "", -1)
			host = strings.TrimSpace(host)
		}
	}
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
	o.Logger.MsgOneLine(common.NoType, "根据 JC(JumpServer config) 配置文件，检查 JumpServer Redis 是否可连接...")
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
	if err := o.CheckMySQL(); err != nil {
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
		o.Logger.Debug(answer)
		if bytePassword, err := terminal.ReadPassword(int(syscall.Stdin)); err != nil {
			o.Logger.Error("输入有误!")
		} else {
			password = string(bytePassword)
			fmt.Println()
			break
		}
	}
	return password
}

func (o *Options) CheckMachine() error {
	if o.MachineInfoPath == "" {
		return fmt.Errorf("待巡检机器文件路径不能为空")
	}
	if _, err := os.Stat(o.MachineInfoPath); err != nil {
		return fmt.Errorf("请检查文件路径: [%s]，文件不存在", o.MachineInfoPath)
	}

	file, err := os.Open(o.MachineInfoPath)
	if err != nil {
		return fmt.Errorf("请检查文件路径: [%s]，文件不存在", o.MachineInfoPath)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	o.Logger.Debug("正在检查模板文件中机器是否有效...")
	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("读取机器模板文件[%s]失败: [%v]", o.MachineInfoPath, err)
	}

	tableTitle := []string{"名称", "类型", "主机地址", "主机端口", "主机用户名", "提权方式", "是否有效"}
	table, tableErr := gotable.Create(tableTitle...)
	if tableErr != nil {
		return fmt.Errorf("初始化表格显示器失败: [%v]", err)
	}
	var invalidMachines []Machine
	machineNameSet := make(map[string]bool)
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
		username, password, valid := row[usernameIdx], row[passwordIdx], "×"
		var privilegeType, privilegePwd = "无", ""
		if privilegeTypeIdx != -1 {
			privilegeType = row[privilegeTypeIdx]
		}
		if privilegePwdIdx != -1 {
			privilegePwd = row[privilegePwdIdx]
		}
		if password == "" {
			title := fmt.Sprintf("请输入主机为 %s([%s])，用户名 [%s] 的密码：", name, host, username)
			password = o.getPasswordFromUser(title)
		}
		if privilegeType != "无" && privilegePwd == "" {
			title := fmt.Sprintf("请输入主机为 %s([%s])，root 的密码：", name, host)
			privilegePwd = o.getPasswordFromUser(title)
		}
		machine := Machine{
			Name: name, Type: strings.ToLower(type_), Host: host, Port: port,
			Username: username, Password: password, PriType: privilegeType, PriPwd: privilegePwd,
		}
		if _, ok := machineNameSet[name]; ok {
			return fmt.Errorf("待巡检机器名称重复，名称为: %s", name)
		} else {
			machineNameSet[name] = true
		}
		o.Logger.MsgOneLine(common.NoType, "\t%v: 正在检查机器 %s([%s]) 是否可连接...", index, machine.Name, machine.Host)
		if err = machine.Connect(); err == nil {
			machine.Valid = true
			o.MachineSet = append(o.MachineSet, machine)
			valid = "✔"
		} else {
			machine.Valid = false
			valid = "×"
			invalidMachines = append(invalidMachines, machine)
		}
		_ = table.AddRow([]string{
			name, type_, host, port, username, privilegeType, valid,
		})
	}
	o.Logger.MsgOneLine(common.Success, "机器检查完成，具体如下：")
	if len(o.MachineSet) == 0 {
		fmt.Printf("\n%s\n", table)
		return fmt.Errorf("没有获取到有效的机器信息，请检查此文件内容: %s", o.MachineInfoPath)
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
