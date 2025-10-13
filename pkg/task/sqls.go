package task

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"inspect/pkg/common"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type TableInfo struct {
	TableName   string
	TableRecord string
	TableSize   string
}

type PieItem struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type ChartCoordinate struct {
	X     string
	Y     string
	XList []string
	YList []string
}

type RDSInfo struct {
	Name  string
	Value any
}

type RDSClient interface {
	Close() error
	Ping() error
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)

	GetRawRdsInfo() map[string]string
	GetTableInfo() ([]TableInfo, error)
	GetRDSInfo() ([]RDSInfo, error)
	GetMaxLoginCount() string
	GetMaxLoginAssetCount() string
	GetMaxLoginUsersInLast3Months() string
	GetMaxAssetLoginsInLast3Months() string
	GetUserLoginsInLastXMonths(months int) string
	GetAssetLoginsInLastXMonths(months int) string
	GetFTPLogsInLastXMonths(months int) string
	GetCommandCountInLastXMonths(months, level int) string
	GetMaxDurationInLastXMonths(months int) string
	GetAvgDurationInLastXMonths(months int) string
	GetTicketCountInLastXMonths(months int) string
	GetUserLoginChart() *ChartCoordinate
	GetAssetLoginChart() *ChartCoordinate
	GetActiveUserChart() *ChartCoordinate
	GetActiveAssetChart() *ChartCoordinate
	GetProtocolsAccessPie() string
}

type RDSBaseClient struct {
	*sql.DB

	DBName  string
	rdsInfo map[string]string
}

func (c *RDSBaseClient) GetRawRdsInfo() map[string]string {
	return c.rdsInfo
}

func (c *RDSBaseClient) Ping() error {
	return c.DB.Ping()
}

func (c *RDSBaseClient) dbInfoGet(key, input string) string {
	if v, exist := c.rdsInfo[key]; exist {
		return v
	} else {
		return input
	}
}

func (c *RDSBaseClient) getChartCoordinate(query string) *ChartCoordinate {
	var err error
	var data []byte
	coordinate := ChartCoordinate{XList: []string{}, YList: []string{}}
	rows, err := c.Query(query)
	if err != nil {
		return &coordinate
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	for rows.Next() {
		var x, y string
		err := rows.Scan(&x, &y)
		if err != nil {
			continue
		}
		coordinate.XList = append(coordinate.XList, x)
		coordinate.YList = append(coordinate.YList, y)
	}
	if data, err = json.Marshal(coordinate.XList); err != nil {
		coordinate.X = "[]"
	} else {
		coordinate.X = string(data)
	}
	if data, err = json.Marshal(coordinate.YList); err != nil {
		coordinate.Y = "[]"
	} else {
		coordinate.Y = string(data)
	}
	return &coordinate
}

func (c *RDSBaseClient) getProtocolsAccessPieData(query string) string {
	var protocolInfos []PieItem
	var result string
	rows, err := c.Query(query)
	if err == nil {
		defer func(rows *sql.Rows) {
			_ = rows.Close()
		}(rows)

		for rows.Next() {
			var name, value string
			err = rows.Scan(&name, &value)
			if err != nil {
				continue
			}
			protocolInfos = append(protocolInfos, PieItem{
				Name: name, Value: value,
			})
		}
	}
	if data, err := json.Marshal(protocolInfos); err == nil {
		result = string(data)
	} else {
		result = "[]"
	}
	return result
}

func (c *RDSBaseClient) GetVariables(command string) error {
	rows, err := c.Query(command)
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
		c.rdsInfo[name] = value
	}
	return nil
}

func (c *RDSBaseClient) getTableInfo(query string, withDBName bool) ([]TableInfo, error) {
	var tables []TableInfo
	var rows *sql.Rows
	var err error
	if withDBName {
		rows, err = c.Query(query, c.DBName)
	} else {
		rows, err = c.Query(query)
	}
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		_ = rows.Close()
	}(rows)

	for rows.Next() {
		var table TableInfo
		_ = rows.Scan(&table.TableName, &table.TableRecord, &table.TableSize)
		tables = append(tables, table)
	}
	return tables, nil
}

func (c *RDSBaseClient) Close() error {
	return c.DB.Close()
}

func (c *RDSBaseClient) Query(query string, args ...any) (*sql.Rows, error) {
	return c.DB.Query(query, args...)
}

func (c *RDSBaseClient) QueryRow(query string, args ...any) *sql.Row {
	return c.DB.QueryRow(query, args...)
}

type MySQLClient struct {
	version string

	RDSBaseClient
}

func (c *MySQLClient) GetVersion() string {
	if c.version != "" {
		return c.version
	}
	err := c.QueryRow("SELECT VERSION()").Scan(&c.version)
	if err != nil {
		return c.version
	}
	re := regexp.MustCompile(`^\d+\.\d+\.\d+`)
	c.version = re.FindString(c.version)
	return c.version
}

func (c *MySQLClient) GetReplicationCommand() string {
	version := c.GetVersion()
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return "SHOW SLAVE STATUS" // 无法解析时默认使用旧命令
	}

	major, _ := strconv.Atoi(parts[0])
	minor, _ := strconv.Atoi(parts[1])
	patch := 0
	if len(parts) > 2 {
		patch, _ = strconv.Atoi(parts[2])
	}

	if (major > 8) ||
		(major == 8 && minor > 0) ||
		(major == 8 && minor == 0 && patch >= 22) {
		return "SHOW REPLICA STATUS"
	}
	return "SHOW SLAVE STATUS"
}

func (c *MySQLClient) GetTableInfo() ([]TableInfo, error) {
	query := "SELECT table_name, table_rows, " +
		"CONCAT(ROUND(data_length/1024/1024, 2), 'M') " +
		"FROM information_schema.tables WHERE table_schema = ? " +
		"ORDER BY table_rows DESC LIMIT 10;"
	return c.getTableInfo(query, true)
}

func (c *MySQLClient) GetRDSInfo() ([]RDSInfo, error) {
	var rdsInfos []RDSInfo
	err := c.GetVariables("SHOW GLOBAL VARIABLES")
	if err != nil {
		return nil, err
	}
	err = c.GetVariables("SHOW GLOBAL STATUS")
	if err != nil {
		return nil, err
	}

	// QPS 计算
	upTime := c.dbInfoGet("Uptime", "1")
	questions := c.dbInfoGet("Questions", "0")
	upTimeInt, _ := strconv.Atoi(upTime)
	questionsInt, _ := strconv.Atoi(questions)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "QPS", Value: questionsInt / upTimeInt})
	// TPS 计算
	commit := c.dbInfoGet("Com_commit", "0")
	rollback := c.dbInfoGet("Com_rollback", "0")
	commitInt, _ := strconv.Atoi(commit)
	rollbackInt, _ := strconv.Atoi(rollback)
	tps := (commitInt + rollbackInt) / upTimeInt
	rdsInfos = append(rdsInfos, RDSInfo{Name: "TPS", Value: tps})
	// 获取slave信息
	dbSlaveSqlRunning := common.Empty
	dbSlaveIORunning := common.Empty
	rows, err := c.Query(c.GetReplicationCommand())
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		columns, err := rows.Columns()
		if err != nil {
			return nil, err
		}
		valuePointers := make([]interface{}, len(columns))
		for i := range valuePointers {
			var value interface{}
			valuePointers[i] = &value
		}
		if err = rows.Scan(valuePointers...); err != nil {
			continue
		}
		for i, name := range columns {
			value := *(valuePointers[i].(*interface{}))
			switch v := value.(type) {
			case []byte:
				value = string(v)
			}
			switch name {
			case "Slave_SQL_Running":
				dbSlaveSqlRunning = fmt.Sprintf("%v", value)
			case "Slave_IO_Running":
				dbSlaveIORunning = fmt.Sprintf("%v", value)
			}
		}
	}
	rdsInfos = append(rdsInfos, RDSInfo{Name: "Slave IO Running", Value: dbSlaveIORunning})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "Slave SQL Running", Value: dbSlaveSqlRunning})
	// 获取表数量
	var tableCount string
	query := "SELECT COUNT(*) FROM information_schema.tables WHERE table_type='BASE TABLE'"
	_ = c.QueryRow(query).Scan(&tableCount)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "总表数", Value: tableCount})
	// 获取当前事务数量
	var trxQueryCount string
	query = "SELECT count(*) FROM information_schema.innodb_trx"
	_ = c.QueryRow(query).Scan(&trxQueryCount)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "当前事务数", Value: trxQueryCount})
	// 其他
	rdsInfos = append(rdsInfos, RDSInfo{Name: "数据库运行时长", Value: common.SecondDisplay(upTimeInt)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "SQL MODE", Value: c.dbInfoGet("sql_mode", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "最大连接数", Value: c.dbInfoGet("max_connections", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "当前连接数", Value: c.dbInfoGet("Threads_connected", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "慢查询数", Value: c.dbInfoGet("slow_query_log", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "字符集", Value: c.dbInfoGet("character_set_database", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "排序规则", Value: c.dbInfoGet("collation_database", common.Empty)})
	return rdsInfos, nil
}

func (c *MySQLClient) buildDateResult(date, count string) string {
	var result string
	if count == "" {
		result = "0"
	} else {
		result = fmt.Sprintf("%s (%s)", count, date)
	}
	return result
}

func (c *MySQLClient) GetMaxLoginCount() string {
	var date, count string
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status=true GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)

	return c.buildDateResult(date, count)
}

func (c *MySQLClient) GetMaxLoginAssetCount() string {
	var date, count string
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *MySQLClient) GetMaxLoginUsersInLast3Months() string {
	var date, count string
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *MySQLClient) GetMaxAssetLoginsInLast3Months() string {
	var date, count string
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *MySQLClient) GetUserLoginsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(DISTINCT username) FROM audits_userloginlog "+
		"WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *MySQLClient) GetAssetLoginsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM terminal_session "+
		"WHERE date_start > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *MySQLClient) GetFTPLogsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' "+
		"AND date_start > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *MySQLClient) GetCommandCountInLastXMonths(months, level int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM terminal_command WHERE "+
		"FROM_UNIXTIME(timestamp) > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	if level != 0 {
		query += fmt.Sprintf(" AND risk_level = %d", level)
	}
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *MySQLClient) GetMaxDurationInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT timediff(date_end, date_start) AS duration from terminal_session "+
		"WHERE date_start > DATE_SUB(CURDATE(), INTERVAL %v MONTH) "+
		"ORDER BY duration DESC LIMIT 1", months)
	_ = c.QueryRow(query).Scan(&duration)
	return duration
}

func (c *MySQLClient) GetAvgDurationInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT ROUND(AVG(TIME_TO_SEC(TIMEDIFF(date_end, date_start))), 0) AS duration "+
		"FROM terminal_session WHERE date_start > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	_ = c.QueryRow(query).Scan(&duration)
	return duration
}

func (c *MySQLClient) GetTicketCountInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT COUNT(*) FROM tickets_ticket "+
		"WHERE date_created > DATE_SUB(CURDATE(), INTERVAL %v MONTH)", months)
	_ = c.QueryRow(query).Scan(&duration)
	return duration
}

func (c *MySQLClient) GetUserLoginChart() *ChartCoordinate {
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= datetime GROUP BY d"
	return c.getChartCoordinate(query)
}

func (c *MySQLClient) GetAssetLoginChart() *ChartCoordinate {
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"WHERE DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= date_start GROUP BY d"
	return c.getChartCoordinate(query)
}

func (c *MySQLClient) GetActiveUserChart() *ChartCoordinate {
	query := "SELECT username, count(*) AS num FROM audits_userloginlog " +
		"WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 1 MONTH) <= datetime " +
		"GROUP BY username ORDER BY num DESC LIMIT 5;"
	return c.getChartCoordinate(query)
}

func (c *MySQLClient) GetActiveAssetChart() *ChartCoordinate {
	query := "SELECT asset, count(*) AS num FROM terminal_session " +
		"WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
		"GROUP BY asset ORDER BY num DESC LIMIT 5;"
	return c.getChartCoordinate(query)
}

func (c *MySQLClient) GetProtocolsAccessPie() string {
	query := "SELECT protocol, count(*) AS num FROM terminal_session " +
		"WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
		"GROUP BY protocol ORDER BY num DESC"
	return c.getProtocolsAccessPieData(query)
}

type PostgreSQLClient struct {
	RDSBaseClient
}

func (c *PostgreSQLClient) GetTableInfo() ([]TableInfo, error) {
	query := "SELECT relname, " +
		"(SELECT COUNT(*) FROM information_schema.columns WHERE table_name = c.relname), " +
		"pg_size_pretty(pg_total_relation_size(relid)) as total_size " +
		"FROM pg_stat_user_tables c ORDER BY total_size DESC LIMIT 10"
	return c.getTableInfo(query, false)
}

func (c *PostgreSQLClient) GetRDSInfo() ([]RDSInfo, error) {
	var rdsInfos []RDSInfo
	err := c.GetVariables("SELECT name, setting FROM pg_settings")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf(
		"SELECT EXTRACT(EPOCH FROM NOW() - pg_postmaster_start_time())::integer, "+
			"xact_commit + xact_rollback, tup_returned + tup_fetched "+
			"FROM pg_stat_database WHERE datname = '%s'", c.DBName,
	)
	// QPS、TPS 计算
	var upTime, questions, transactions int
	_ = c.QueryRow(query).Scan(&upTime, &questions, &transactions)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "QPS", Value: questions / upTime})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "TPS", Value: transactions / upTime})
	// 主从状态
	var syncState, state, backendStart, replicationLag string
	query = "SELECT sync_state, state, backend_start, pg_wal_lsn_diff(pg_current_wal_lsn(), replay_lsn) FROM pg_stat_replication"
	_ = c.QueryRow(query).Scan(&syncState, &state, &backendStart, &replicationLag)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "同步模式", Value: common.InputOrEmpty(syncState)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "同步状态", Value: common.InputOrEmpty(state)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "复制进程启动时间", Value: common.InputOrEmpty(backendStart)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "复制延迟(MB)", Value: common.InputOrEmpty(replicationLag)})
	// 获取表数量
	var tableCount string
	query = "SELECT count(*) FROM information_schema.tables where table_schema = 'public'"
	_ = c.QueryRow(query).Scan(&tableCount)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "总表数", Value: tableCount})
	// 获取当前事务数量
	var trxQueryCount string
	query = "SELECT count(*) FROM pg_stat_activity where state = 'active'"
	_ = c.QueryRow(query).Scan(&trxQueryCount)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "当前事务数", Value: trxQueryCount})
	// 其他
	var upTimeDisplay string
	query = "SELECT date_trunc('second', current_timestamp - pg_postmaster_start_time())"
	_ = c.QueryRow(query).Scan(&upTimeDisplay)
	rdsInfos = append(rdsInfos, RDSInfo{Name: "版本", Value: c.dbInfoGet("server_version", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "数据库运行时长", Value: upTimeDisplay})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "最大连接数", Value: c.dbInfoGet("max_connections", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "事务锁超时时间", Value: c.dbInfoGet("lock_timeout", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "单索引最大列数", Value: c.dbInfoGet("max_index_keys", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "单事务最大持有锁数", Value: c.dbInfoGet("max_locks_per_transaction", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "字符编码", Value: c.dbInfoGet("server_encoding", common.Empty)})
	rdsInfos = append(rdsInfos, RDSInfo{Name: "时区", Value: c.dbInfoGet("TimeZone", common.Empty)})
	return rdsInfos, nil
}

func (c *PostgreSQLClient) buildDateResult(date time.Time, count string) string {
	var result string
	if count == "" {
		result = "0"
	} else {
		result = fmt.Sprintf("%s (%s)", count, date.Format("2006-01-02"))
	}
	return result
}

func (c *PostgreSQLClient) GetMaxLoginCount() string {
	var date time.Time
	var count string
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status=true GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *PostgreSQLClient) GetMaxLoginAssetCount() string {
	var date time.Time
	var count string
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *PostgreSQLClient) GetMaxLoginUsersInLast3Months() string {
	var date time.Time
	var count string
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status = true AND datetime > CURRENT_DATE - INTERVAL '3 months' " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *PostgreSQLClient) GetMaxAssetLoginsInLast3Months() string {
	var date time.Time
	var count string
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"WHERE date_start > CURRENT_DATE - INTERVAL '3 months' " +
		"GROUP BY d ORDER BY num DESC LIMIT 1"
	_ = c.QueryRow(query).Scan(&date, &count)
	return c.buildDateResult(date, count)
}

func (c *PostgreSQLClient) GetUserLoginsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(DISTINCT username) FROM audits_userloginlog "+
		"WHERE status=true AND datetime > CURRENT_DATE - INTERVAL '%v month'", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *PostgreSQLClient) GetAssetLoginsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM terminal_session "+
		"WHERE date_start > CURRENT_DATE - INTERVAL '%v month'", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *PostgreSQLClient) GetFTPLogsInLastXMonths(months int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' "+
		"AND date_start > CURRENT_DATE - INTERVAL '%v month'", months)
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *PostgreSQLClient) GetCommandCountInLastXMonths(months, level int) string {
	var count string
	query := fmt.Sprintf("SELECT COUNT(*) FROM terminal_command WHERE "+
		"to_timestamp(timestamp) > CURRENT_DATE - INTERVAL '%v month'", months)
	if level != 0 {
		query += fmt.Sprintf(" AND risk_level = %d", level)
	}
	_ = c.QueryRow(query).Scan(&count)
	return count
}

func (c *PostgreSQLClient) GetMaxDurationInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT date_end - date_start AS duration FROM terminal_session "+
		"WHERE date_start > CURRENT_DATE - INTERVAL '%v month' "+
		"ORDER BY duration DESC LIMIT 1", months)
	_ = c.QueryRow(query).Scan(&duration)
	return duration
}

func (c *PostgreSQLClient) GetAvgDurationInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT ROUND(AVG(EXTRACT(EPOCH FROM (date_end - date_start))), 0) AS duration "+
		"FROM terminal_session WHERE date_start > CURRENT_DATE - INTERVAL '%v month'", months)
	err := c.QueryRow(query).Scan(&duration)
	fmt.Print(err)
	return duration
}

func (c *PostgreSQLClient) GetTicketCountInLastXMonths(months int) string {
	var duration string
	query := fmt.Sprintf("SELECT COUNT(*) FROM tickets_ticket "+
		"WHERE date_created > CURRENT_DATE - INTERVAL '%v month'", months)
	_ = c.QueryRow(query).Scan(&duration)
	return duration
}

func (c *PostgreSQLClient) GetUserLoginChart() *ChartCoordinate {
	query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
		"WHERE status=true and CURRENT_DATE - INTERVAL '6 day' <= datetime GROUP BY d"
	return c.getChartCoordinate(query)
}

func (c *PostgreSQLClient) GetAssetLoginChart() *ChartCoordinate {
	query := "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
		"WHERE CURRENT_DATE - INTERVAL '6 DAY' <= date_start GROUP BY d"
	return c.getChartCoordinate(query)
}

func (c *PostgreSQLClient) GetActiveUserChart() *ChartCoordinate {
	query := "SELECT username, count(*) AS num FROM audits_userloginlog " +
		"WHERE status=true and CURRENT_DATE - INTERVAL '1 month' <= datetime " +
		"GROUP BY username ORDER BY num DESC LIMIT 5"
	return c.getChartCoordinate(query)
}

func (c *PostgreSQLClient) GetActiveAssetChart() *ChartCoordinate {
	query := "SELECT asset, count(*) AS num FROM terminal_session " +
		"WHERE CURRENT_DATE - INTERVAL '3 month' <= date_start " +
		"GROUP BY asset ORDER BY num DESC LIMIT 5"
	return c.getChartCoordinate(query)
}

func (c *PostgreSQLClient) GetProtocolsAccessPie() string {
	query := "SELECT protocol, count(*) AS num FROM terminal_session " +
		"WHERE CURRENT_DATE - INTERVAL '3 month' <= date_start " +
		"GROUP BY protocol ORDER BY num DESC"
	return c.getProtocolsAccessPieData(query)
}

func newSQLClient(engine, dbName string, db *sql.DB) RDSClient {
	if engine == common.PostgreSQL {
		return &PostgreSQLClient{
			RDSBaseClient: RDSBaseClient{
				rdsInfo: make(map[string]string),
				DB:      db,
				DBName:  dbName,
			},
		}
	}
	return &MySQLClient{
		RDSBaseClient: RDSBaseClient{
			rdsInfo: make(map[string]string),
			DB:      db,
			DBName:  dbName,
		},
	}
}
