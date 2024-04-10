package task

import (
    "database/sql"
    "fmt"
    "github.com/go-redis/redis"
    "inspect/pkg/common"
    "strconv"
    "strings"
)

type TableInfo struct {
    TableName   string
    TableRecord string
    TableSize   string
}

type DBTask struct {
    Task

    mysqlClient *sql.DB
    redisClient *redis.Client

    redisInfo map[string]string
    mysqlInfo map[string]string
}

func (t *DBTask) Init(opts *Options) error {
    t.Options = opts
    t.result = make(map[string]interface{})
    t.redisClient = opts.GetRedisClient()

    client, err := opts.GetMySQLClient()
    if err != nil {
        return err
    }
    t.mysqlInfo = make(map[string]string)
    t.mysqlClient = client
    return nil
}

func (t *DBTask) Get(key string) string {
    if v, exist := t.redisInfo[key]; exist {
        return v
    } else {
        return common.Empty
    }
}

func (t *DBTask) dbGet(key, input string) string {
    if v, exist := t.mysqlInfo[key]; exist {
        return v
    } else {
        return input
    }
}

func (t *DBTask) GetVariables(command string) error {
    rows, err := t.mysqlClient.Query(command)
    if err != nil {
        return err
    }
    defer func(rows *sql.Rows) {
        _ = rows.Close()
    }(rows)

    for rows.Next() {
        var name, value string
        err = rows.Scan(&name, &value)
        if err != nil {
            continue
        }
        t.mysqlInfo[name] = value
    }
    return nil
}

func (t *DBTask) GetTableInfo() error {
    query := "SELECT table_name, table_rows, " +
        "ROUND(data_length/1024/1024, 2) " +
        "FROM information_schema.tables WHERE table_schema = ? " +
        "ORDER BY table_rows DESC LIMIT 10;"

    var tables []TableInfo
    rows, err := t.mysqlClient.Query(query, t.GetConfig("DB_NAME", "JUMPSERVER"))
    if err != nil {
        return err
    }
    defer func(rows *sql.Rows) {
        _ = rows.Close()
    }(rows)

    for rows.Next() {
        var table TableInfo
        _ = rows.Scan(&table.TableName, &table.TableRecord, &table.TableSize)
        tables = append(tables, table)
    }
    t.result["Top10Table"] = tables
    return nil
}

func (t *DBTask) GetDBInfo() error {
    err := t.GetVariables("SHOW GLOBAL VARIABLES")
    if err != nil {
        return err
    }
    err = t.GetVariables("SHOW GLOBAL STATUS")
    if err != nil {
        return err
    }

    // QPS 计算
    upTime := t.dbGet("Uptime", "1")
    questions := t.dbGet("Questions", "0")
    upTimeInt, _ := strconv.Atoi(upTime)
    questionsInt, _ := strconv.Atoi(questions)
    t.result["DBQps"] = questionsInt / upTimeInt
    // TPS 计算
    commit := t.dbGet("Com_commit", "0")
    rollback := t.dbGet("Com_rollback", "0")
    commitInt, _ := strconv.Atoi(commit)
    rollbackInt, _ := strconv.Atoi(rollback)
    tps := (commitInt + rollbackInt) / upTimeInt
    t.result["DBTps"] = tps
    // 获取slave信息
    dbSlaveSqlRunning := common.Empty
    dbSlaveIORunning := common.Empty
    rows, err := t.mysqlClient.Query("SHOW SLAVE STATUS")
    if err != nil {
        return err
    }
    for rows.Next() {
        columns, err := rows.Columns()
        valuePointers := make([]interface{}, len(columns))
        if err != nil {
            return err
        }
        if err = rows.Scan(valuePointers...); err != nil {
            continue
        }
        for i, name := range columns {
            switch name {
            case "Slave_SQL_Running":
                dbSlaveSqlRunning = fmt.Sprintf("%v", valuePointers[i])
            case "Slave_IO_Running":
                dbSlaveIORunning = fmt.Sprintf("%v", valuePointers[i])
            }
        }
    }
    t.result["DBSlaveIORunning"] = dbSlaveIORunning
    t.result["DBSlaveSqlRunning"] = dbSlaveSqlRunning
    // 获取表数量
    var tableCount string
    tableCountQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_type='BASE TABLE'"
    _ = t.mysqlClient.QueryRow(tableCountQuery).Scan(&tableCount)
    t.result["DBTableCount"] = tableCount
    // 获取当前事务数量
    var trxQueryCount string
    trxQuery := "SELECT count(*) FROM information_schema.innodb_trx"
    _ = t.mysqlClient.QueryRow(trxQuery).Scan(&trxQueryCount)
    t.result["DBCurrentTransaction"] = trxQueryCount
    // 其他
    t.result["DBOperatingTime"] = common.SecondDisplay(upTimeInt)
    t.result["DBSqlMode"] = t.dbGet("sql_mode", common.Empty)
    t.result["DBMaxConnect"] = t.dbGet("max_connections", common.Empty)
    t.result["DBCurrentConnect"] = t.dbGet("Threads_connected", common.Empty)
    t.result["DBSlowQuery"] = t.dbGet("slow_query_log", common.Empty)
    t.result["DBCharset"] = t.dbGet("character_set_database", common.Empty)
    t.result["DBSortRule"] = t.dbGet("collation_database", common.Empty)
    return nil
}

func (t *DBTask) GetMySQLInfo() error {
    t.result["HasMySQLInfo"] = true

    if err := t.GetTableInfo(); err != nil {
        return err
    }
    if err := t.GetDBInfo(); err != nil {
        return err
    }
    return nil
}

func (t *DBTask) SetRedisInfoFromServer() error {
    infoStr, err := t.redisClient.Info().Result()
    if err != nil {
        return fmt.Errorf("获取 Redis 的 info 信息失败: %s", err)
    }
    info := make(map[string]string)
    lines := strings.Split(infoStr, "\n")
    for _, line := range lines {
        if line != "" && !strings.HasPrefix(line, "#") {
            parts := strings.Split(line, ":")
            if len(parts) == 2 {
                info[parts[0]] = parts[1]
            }
        }
    }
    t.redisInfo = info
    return nil
}

func (t *DBTask) GetRedisInfo() error {
    err := t.SetRedisInfoFromServer()
    if err != nil {
        return err
    }

    t.result["HasRedisInfo"] = true
    // service info
    t.result["RedisVersion"] = t.Get("redis_version")
    t.result["RedisMode"] = t.Get("redis_mode")
    t.result["RedisPort"] = t.Get("tcp_port")
    t.result["RedisUptime"] = t.Get("uptime_in_days")

    // client info
    t.result["RedisConnect"] = t.Get("connected_clients")
    t.result["RedisClusterConnect"] = t.Get("cluster_connections")
    t.result["RedisMaxConnect"] = t.Get("maxclients")
    t.result["RedisBlockedConnect"] = t.Get("blocked_clients")

    // memory info
    t.result["UsedMemoryHuman"] = t.Get("used_memory_human")
    t.result["UsedMemoryRssHuman"] = t.Get("used_memory_rss_human")
    t.result["UsedMemoryPeakHuman"] = t.Get("used_memory_peak_human")
    t.result["UsedMemoryLuaHuman"] = t.Get("used_memory_lua_human")
    t.result["MaxMemoryHuman"] = t.Get("maxmemory_human")
    t.result["MaxMemoryPolicy"] = t.Get("maxmemory_policy")

    // statistics info
    t.result["TotalConnectionsReceived"] = t.Get("total_connections_received")
    t.result["TotalCommandsProcessed"] = t.Get("total_commands_processed")
    t.result["InstantaneousOpsPerSec"] = t.Get("instantaneous_ops_per_sec")
    t.result["TotalNetInputBytes"] = t.Get("total_net_input_bytes")
    t.result["TotalNetOutputBytes"] = t.Get("total_net_output_bytes")
    t.result["RejectedConnections"] = t.Get("rejected_connections")
    t.result["ExpiredKeys"] = t.Get("expired_keys")
    t.result["EvictedKeys"] = t.Get("evicted_keys")
    t.result["KeyspaceHits"] = t.Get("keyspace_hits")
    t.result["KeyspaceMisses"] = t.Get("keyspace_misses")
    t.result["PubSubChannels"] = t.Get("pubsub_channels")
    t.result["PubSubPatterns"] = t.Get("pubsub_patterns")

    return nil
}

func (t *DBTask) GetName() string {
    return "数据库"
}

func (t *DBTask) Run() error {
    if err := t.GetMySQLInfo(); err != nil {
        return err
    }
    if err := t.GetRedisInfo(); err != nil {
        return err
    }
    return nil
}
