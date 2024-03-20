package task

import (
    "database/sql"
    "fmt"
    "strconv"

    "inspect/pkg/common"
)

type TableInfo struct {
    TableName   string
    TableRecord string
    TableSize   string
}

type MySQLTask struct {
    Task

    DBInfo map[string]string
    client *sql.DB
}

func (t *MySQLTask) Init(opts *Options) error {
    var err error
    if client, err := opts.GetMySQLClient(); err == nil {
        t.Options = opts
        t.result = make(map[string]interface{})
        t.DBInfo = make(map[string]string)
        t.client = client
    }
    return err
}

func (t *MySQLTask) Get(key, input string) string {
    if v, exist := t.DBInfo[key]; exist {
        return v
    } else {
        return input
    }
}

func (t *MySQLTask) GetTableInfo() error {
    query := "SELECT table_name, table_rows, " +
        "ROUND(data_length/1024/1024, 2) " +
        "FROM information_schema.tables WHERE table_schema = ? " +
        "ORDER BY table_rows DESC LIMIT 10;"

    var tables []TableInfo
    rows, err := t.client.Query(query, t.GetConfig("DB_NAME", "JUMPSERVER"))
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

func (t *MySQLTask) GetVariables(command string) error {
    rows, err := t.client.Query(command)
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
        t.DBInfo[name] = value
    }
    return nil
}

func (t *MySQLTask) GetDBInfo() error {
    err := t.GetVariables("SHOW GLOBAL VARIABLES")
    if err != nil {
        return err
    }
    err = t.GetVariables("SHOW GLOBAL STATUS")
    if err != nil {
        return err
    }

    // QPS 计算
    upTime := t.Get("Uptime", "1")
    questions := t.Get("Questions", "0")
    upTimeInt, _ := strconv.Atoi(upTime)
    questionsInt, _ := strconv.Atoi(questions)
    t.result["db_qps"] = questionsInt / upTimeInt
    // TPS 计算
    commit := t.Get("Com_commit", "0")
    rollback := t.Get("Com_rollback", "0")
    commitInt, _ := strconv.Atoi(commit)
    rollbackInt, _ := strconv.Atoi(rollback)
    tps := (commitInt + rollbackInt) / upTimeInt
    t.result["db_tps"] = tps
    // 获取slave信息
    dbSlaveSqlRunning := common.Empty
    dbSlaveIORunning := common.Empty
    rows, err := t.client.Query("SHOW SLAVE STATUS")
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
    _ = t.client.QueryRow(tableCountQuery).Scan(&tableCount)
    t.result["db_table_count"] = tableCount
    // 获取当前事务数量
    var trxQueryCount string
    trxQuery := "SELECT count(*) FROM information_schema.innodb_trx"
    _ = t.client.QueryRow(trxQuery).Scan(&trxQueryCount)
    t.result["db_current_transaction"] = trxQueryCount
    // 其他
    t.result["db_operating_time"] = common.SecondDisplay(upTimeInt)
    t.result["db_sql_mode"] = t.Get("sql_mode", common.Empty)
    t.result["db_max_connect"] = t.Get("max_connections", common.Empty)
    t.result["db_current_connect"] = t.Get("Threads_connected", common.Empty)
    t.result["db_slow_query"] = t.Get("slow_query_log", common.Empty)
    t.result["db_charset"] = t.Get("character_set_database", common.Empty)
    t.result["db_sort_rule"] = t.Get("collation_database", common.Empty)
    return nil
}

func (t *MySQLTask) GetName() string {
    return "堡垒机后端 MySQL 检查"
}

func (t *MySQLTask) Run() error {
    err := t.GetTableInfo()
    if err != nil {
        return err
    }
    if err = t.GetDBInfo(); err != nil {
        return err
    }
    return nil
}
