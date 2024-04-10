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
    t.result["top_10_table"] = tables
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
    t.result["db_qps"] = questionsInt / upTimeInt
    // TPS 计算
    commit := t.dbGet("Com_commit", "0")
    rollback := t.dbGet("Com_rollback", "0")
    commitInt, _ := strconv.Atoi(commit)
    rollbackInt, _ := strconv.Atoi(rollback)
    tps := (commitInt + rollbackInt) / upTimeInt
    t.result["db_tps"] = tps
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
    t.result["db_slave_io_running"] = dbSlaveIORunning
    t.result["db_slave_sql_running"] = dbSlaveSqlRunning
    // 获取表数量
    var tableCount string
    tableCountQuery := "SELECT COUNT(*) FROM information_schema.tables WHERE table_type='BASE TABLE'"
    _ = t.mysqlClient.QueryRow(tableCountQuery).Scan(&tableCount)
    t.result["db_table_count"] = tableCount
    // 获取当前事务数量
    var trxQueryCount string
    trxQuery := "SELECT count(*) FROM information_schema.innodb_trx"
    _ = t.mysqlClient.QueryRow(trxQuery).Scan(&trxQueryCount)
    t.result["db_current_transaction"] = trxQueryCount
    // 其他
    t.result["db_operating_time"] = common.SecondDisplay(upTimeInt)
    t.result["db_sql_mode"] = t.dbGet("sql_mode", common.Empty)
    t.result["db_max_connect"] = t.dbGet("max_connections", common.Empty)
    t.result["db_current_connect"] = t.dbGet("Threads_connected", common.Empty)
    t.result["db_slow_query"] = t.dbGet("slow_query_log", common.Empty)
    t.result["db_charset"] = t.dbGet("character_set_database", common.Empty)
    t.result["db_sort_rule"] = t.dbGet("collation_database", common.Empty)
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
    t.result["redis_version"] = t.Get("redis_version")
    t.result["redis_mode"] = t.Get("redis_mode")
    t.result["redis_port"] = t.Get("tcp_port")
    t.result["redis_uptime"] = t.Get("uptime_in_days")

    // client info
    t.result["redis_connect"] = t.Get("connected_clients")
    t.result["redis_cluster_connect"] = t.Get("cluster_connections")
    t.result["redis_max_connect"] = t.Get("maxclients")
    t.result["redis_blocked_connect"] = t.Get("blocked_clients")

    // memory info
    t.result["used_memory_human"] = t.Get("used_memory_human")
    t.result["used_memory_rss_human"] = t.Get("used_memory_rss_human")
    t.result["used_memory_peak_human"] = t.Get("used_memory_peak_human")
    t.result["used_memory_lua_human"] = t.Get("used_memory_lua_human")
    t.result["maxmemory_human"] = t.Get("maxmemory_human")
    t.result["maxmemory_policy"] = t.Get("maxmemory_policy")

    // statistics info
    t.result["total_connections_received"] = t.Get("total_connections_received")
    t.result["total_commands_processed"] = t.Get("total_commands_processed")
    t.result["instantaneous_ops_per_sec"] = t.Get("instantaneous_ops_per_sec")
    t.result["total_net_input_bytes"] = t.Get("total_net_input_bytes")
    t.result["total_net_output_bytes"] = t.Get("total_net_output_bytes")
    t.result["rejected_connections"] = t.Get("rejected_connections")
    t.result["expired_keys"] = t.Get("expired_keys")
    t.result["evicted_keys"] = t.Get("evicted_keys")
    t.result["keyspace_hits"] = t.Get("keyspace_hits")
    t.result["keyspace_misses"] = t.Get("keyspace_misses")
    t.result["pubsub_channels"] = t.Get("pubsub_channels")
    t.result["pubsub_patterns"] = t.Get("pubsub_patterns")

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
