package task

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"
)

type SummaryTask struct {
    Task

    Client *sql.DB
}

type ChartCoordinate struct {
    X     string
    Y     string
    XList []string
    YList []string
}

type PieItem struct {
    Name  string `json:"name"`
    Value string `json:"value"`
}

func (t *SummaryTask) getOne(query string) string {
    count := "0"
    _ = t.Client.QueryRow(query).Scan(&count)
    return count
}

func (t *SummaryTask) getTwo(query string) (string, string) {
    var one, two string
    _ = t.Client.QueryRow(query).Scan(&one, &two)
    return one, two
}

func (t *SummaryTask) getChartCoordinate(query string) *ChartCoordinate {
    var err error
    var data []byte
    coordinate := ChartCoordinate{XList: []string{}, YList: []string{}}
    rows, err := t.Client.Query(query)
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

func (t *SummaryTask) GetJMSSummary() {
    // 获取用户总数
    query := "SELECT COUNT(*) FROM users_user WHERE is_service_account=0"
    t.result["user_count"] = t.getOne(query)
    // 获取资产总数
    query = "SELECT COUNT(*) FROM assets_asset"
    t.result["asset_count"] = t.getOne(query)
    // 获取在线会话总数
    query = "SELECT COUNT(*) FROM terminal_session WHERE is_finished=0"
    t.result["online_session"] = t.getOne(query)
    // 获取各平台资产数量
    var display []string
    query = "SELECT p.name, COUNT(*) AS asset_count FROM assets_platform p " +
        "JOIN assets_asset a ON p.id = a.platform_id " +
        "GROUP BY p.name ORDER BY asset_count desc LIMIT 3;"
    rows, err := t.Client.Query(query)
    if err == nil {
        defer func(rows *sql.Rows) {
            _ = rows.Close()
        }(rows)
        var platform, count string
        for rows.Next() {
            err = rows.Scan(&platform, &count)
            if err != nil {
                continue
            }
            display = append(display, fmt.Sprintf(", %s类型：%s个", platform, count))
        }
    }
    t.result["asset_count_display"] = strings.Join(display, "，")
    // 获取组织数量
    query = "SELECT COUNT(*) FROM orgs_organization"
    t.result["organization_count"] = t.getOne(query)
    // 获取最大单日登录次数
    query = "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two := t.getTwo(query)
    if one == "" {
        t.result["max_login_count"] = "0"
    } else {
        t.result["max_login_count"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 最大单日访问资产数
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["max_connect_asset_count"] = "0"
    } else {
        t.result["max_connect_asset_count"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近三月最大单日用户登录数
    query = "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["last_3_month_max_login_count"] = "0"
    } else {
        t.result["last_3_month_max_login_count"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近三月最大单日资产登录数
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM  terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["last_3_month_max_connect_asset_count"] = "0"
    } else {
        t.result["last_3_month_max_connect_asset_count"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近一月登录用户数
    query = "SELECT COUNT(DISTINCT username) FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["last_1_month_login_count"] = t.getOne(query)
    // 近一月登录资产数
    query = "SELECT COUNT(*) FROM terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["last_1_month_connect_asset_count"] = t.getOne(query)
    // 近一月文件上传数
    query = "SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' " +
        "AND date_start > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["last_1_month_upload_count"] = t.getOne(query)
    // 近三月登录用户数
    query = "SELECT COUNT(DISTINCT username) FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_login_count"] = t.getOne(query)
    // 近三月登录资产数
    query = "SELECT COUNT(*) FROM terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_connect_asset_count"] = t.getOne(query)
    // 近三月文件上传数
    query = "SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' " +
        "AND date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_upload_count"] = t.getOne(query)
    // 近三月命令记录数
    query = "SELECT COUNT(*) FROM terminal_command WHERE " +
        "FROM_UNIXTIME(timestamp) > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_command_count"] = t.getOne(query)
    // 近三月高危命令记录数
    query = "SELECT count(*) FROM terminal_command WHERE risk_level=5 and " +
        "FROM_UNIXTIME(timestamp) > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_danger_command_count"] = t.getOne(query)
    // 近三月最大会话时长
    query = "SELECT timediff(date_end, date_start) AS duration from terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "ORDER BY duration DESC LIMIT 1"
    t.result["last_3_month_max_session_duration"] = t.getOne(query)
    // 近三月平均会话时长
    query = "SELECT ROUND(AVG(TIME_TO_SEC(TIMEDIFF(date_end, date_start))), 0) AS duration " +
        "FROM terminal_session WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_avg_session_duration"] = t.getOne(query)
    // 近三月工单申请数
    query = "SELECT COUNT(*) FROM tickets_ticket WHERE date_created > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["last_3_month_ticket_count"] = t.getOne(query)
}

func (t *SummaryTask) GetChartData() {
    // 按周用户登录折线图
    query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= datetime GROUP BY d"
    t.result["user_login_chart"] = t.getChartCoordinate(query)

    // 按周资产登录折线图
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= date_start GROUP BY d"
    t.result["asset_connect_chart"] = t.getChartCoordinate(query)

    // 月活跃用户柱状图
    query = "SELECT username, count(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 1 MONTH) <= datetime " +
        "GROUP BY username ORDER BY num DESC LIMIT 5;"
    t.result["active_user_chart"] = t.getChartCoordinate(query)

    // 近3个月活跃资产柱状图
    query = "SELECT asset, count(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
        "GROUP BY asset ORDER BY num DESC LIMIT 5;"
    t.result["active_asset_chart"] = t.getChartCoordinate(query)

    // 近3个月各种协议访问饼状图
    var protocolInfos []PieItem
    query = "SELECT protocol, count(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
        "GROUP BY protocol ORDER BY num DESC"
    rows, err := t.Client.Query(query)
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
        t.result["protocol_chart"] = string(data)
    } else {
        t.result["protocol_chart"] = "[]"
    }
}

func (t *SummaryTask) GetName() string {
    return "摘要"
}

func (t *SummaryTask) Run() {
    t.GetJMSSummary()
    t.GetChartData()
}
