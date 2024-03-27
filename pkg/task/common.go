package task

import (
    "database/sql"
    "encoding/csv"
    "fmt"
    "golang.org/x/crypto/ssh/terminal"
    "inspect/pkg/common"
    "os"
    "os/exec"
    "strings"
    "syscall"

    "github.com/go-redis/redis"
    _ "github.com/go-sql-driver/mysql"
    "github.com/liushuochen/gotable"
)

type GlobalInfo struct {
    Machines        []Machine
    JMSCount        int
    MySQLCount      int
    RedisCount      int
    TotalCount      int
    InspectDatetime string
}

type ResultSummary struct {
    GlobalInfo GlobalInfo

    NormalResults   []map[string]interface{}
    AbnormalResults []AbnormalMsg
    VirtualResult   map[string]interface{}

    // Other
    EchartsData string
}

func (r *ResultSummary) SetOtherInfo(opts *Options) {
    r.GlobalInfo.InspectDatetime = common.CurrentDatetime(false)
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
    ReportType      string
    JMSConfigPath   string
    MachineInfoPath string

    // 解析的参数
    JMSConfig   map[string]string
    MachineSet  []Machine
    MySQLClient *sql.DB
    RedisClient *redis.Client
}

func (o *Options) Clear() {
    if o.MySQLClient != nil {
        _ = o.MySQLClient.Close()
    }
    if o.RedisClient != nil {
        _ = o.RedisClient.Close()
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
    db, err := o.GetMySQLClient()
    if err != nil {
        return err
    }
    o.MySQLClient = db
    if err = db.Ping(); err != nil {
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
            Password: "Calong@2013",
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
    rdb := o.GetRedisClient()
    o.RedisClient = rdb
    defer func(rdb *redis.Client) {
        _ = rdb.Close()
    }(rdb)
    if _, err := rdb.Ping().Result(); err != nil {
        return fmt.Errorf("连接 JumpServer Redis 失败: %v", err)
    }
    return nil
}

func (o *Options) CheckDB() error {
    o.Logger.Debug("正在根据 JC 配置文件，检查 JumpServer 数据库是否可连接...")
    if err := o.CheckMySQL(); err != nil {
        return err
    }
    if err := o.CheckRedis(); err != nil {
        return err
    }
    return nil
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

    o.Logger.Debug("正在检查模板文件中机器是否合法...")
    reader := csv.NewReader(file)
    rows, err := reader.ReadAll()
    if err != nil {
        return fmt.Errorf("读取机器模板文件[%s]失败: [%v]", o.MachineInfoPath, err)
    }

    tableTitle := []string{"name", "type", "ssh_host", "ssh_port", "ssh_username", "valid"}
    table, tableErr := gotable.Create(tableTitle...)

    var invalidMachines []Machine
    machineNameSet := make(map[string]bool)
    for index, row := range rows {
        if index == 0 {
            continue
        }
        if len(row) != 6 {
            return fmt.Errorf("机器配置填写有误，请检查: [%v]", err)
        }
        name, type_, host, port, username, password, valid := row[0], row[1], row[2], row[3], row[4], row[5], "×"
        if password == "" {
            for i := 1; i < 4; i++ {
                o.Logger.Debug("请输入主机为%s([%s])，用户名[%s]的密码：", name, host, username)
                if bytePassword, err := terminal.ReadPassword(syscall.Stdin); err != nil {
                    o.Logger.Error("输入有误!")
                } else {
                    password = string(bytePassword)
                    fmt.Println()
                    break
                }
            }
        }
        machine := Machine{
            Name: name, Type: strings.ToLower(type_), Host: host, Port: port,
            Username: username, Password: password,
        }
        if _, ok := machineNameSet[name]; ok {
            return fmt.Errorf("待巡检机器名称重复，名称为: %s", name)
        } else {
            machineNameSet[name] = true
        }
        o.Logger.Debug("正在检查机器%s([%s])是否可连接...", machine.Name, machine.Host)
        if machine.Connect() {
            machine.Valid = true
            o.MachineSet = append(o.MachineSet, machine)
            valid = "✔"
        } else {
            machine.Valid = false
            valid = "×"
            invalidMachines = append(invalidMachines, machine)
        }
        tableErr = table.AddRow([]string{
            name, type_, host, port, username, valid,
        })
    }

    if len(o.MachineSet) == 0 {
        return fmt.Errorf("没有获取到机器信息，请检查此文件内容: %s", o.MachineInfoPath)
    }

    if tableErr == nil {
        var answer string
        fmt.Printf("\n%s\n", table)
        fmt.Print("是否继续执行，本次任务只会执行有效资产(默认为 yes): ")
        _, _ = fmt.Scanln(&answer)
        answerStr := strings.ToLower(string(answer))
        if answerStr == "" || answerStr == "y" || answerStr == "yes" {
            return nil
        } else {
            os.Exit(0)
        }
    }
    return nil
}

func (o *Options) Valid() error {
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
