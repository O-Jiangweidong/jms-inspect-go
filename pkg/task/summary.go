package task

import (
    "database/sql"
    "encoding/json"
    "fmt"
    "strings"
)

type SummaryTask struct {
    Task

    client *sql.DB
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

func (t *SummaryTask) Init(opts *Options) error {
    t.Options = opts
    t.result = make(map[string]interface{})
    client, err := opts.GetMySQLClient()
    if err != nil {
        return err
    }
    t.client = client
    return nil
}

func (t *SummaryTask) getOne(query string) string {
    count := "0"
    _ = t.client.QueryRow(query).Scan(&count)
    return count
}

func (t *SummaryTask) getTwo(query string) (string, string) {
    var one, two string
    _ = t.client.QueryRow(query).Scan(&one, &two)
    return one, two
}

func (t *SummaryTask) getChartCoordinate(query string) *ChartCoordinate {
    var err error
    var data []byte
    coordinate := ChartCoordinate{XList: []string{}, YList: []string{}}
    rows, err := t.client.Query(query)
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
    t.result["UserCount"] = t.getOne(query)
    // 获取资产总数
    query = "SELECT COUNT(*) FROM assets_asset"
    t.result["AssetCount"] = t.getOne(query)
    // 获取在线会话总数
    query = "SELECT COUNT(*) FROM terminal_session WHERE is_finished=0"
    t.result["OnlineSession"] = t.getOne(query)
    // 获取各平台资产数量
    var display []string
    query = "SELECT p.name, COUNT(*) AS asset_count FROM assets_platform p " +
        "JOIN assets_asset a ON p.id = a.platform_id " +
        "GROUP BY p.name ORDER BY asset_count desc LIMIT 3;"
    rows, err := t.client.Query(query)
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
    t.result["AssetCountDisplay"] = strings.Join(display, "，")
    // 获取组织数量
    query = "SELECT COUNT(*) FROM orgs_organization"
    t.result["OrganizationCount"] = t.getOne(query)
    // 获取最大单日登录次数
    query = "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two := t.getTwo(query)
    if one == "" {
        t.result["MaxLoginCount"] = "0"
    } else {
        t.result["MaxLoginCount"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 最大单日访问资产数
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["MaxConnectAssetCount"] = "0"
    } else {
        t.result["MaxConnectAssetCount"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近三月最大单日用户登录数
    query = "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["Last3MonthMaxLoginCount"] = "0"
    } else {
        t.result["Last3MonthMaxLoginCount"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近三月最大单日资产登录数
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM  terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "GROUP BY d ORDER BY num DESC LIMIT 1"
    one, two = t.getTwo(query)
    if one == "" {
        t.result["Last3MonthMaxConnectAssetCount"] = "0"
    } else {
        t.result["Last3MonthMaxConnectAssetCount"] = fmt.Sprintf("%s(%s)", one, two)
    }
    // 近一月登录用户数
    query = "SELECT COUNT(DISTINCT username) FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["Last1MonthLoginCount"] = t.getOne(query)
    // 近一月登录资产数
    query = "SELECT COUNT(*) FROM terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["Last1MonthConnectAssetCount"] = t.getOne(query)
    // 近一月文件上传数
    query = "SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' " +
        "AND date_start > DATE_SUB(CURDATE(), INTERVAL 1 MONTH)"
    t.result["Last1MonthUploadCount"] = t.getOne(query)
    // 近三月登录用户数
    query = "SELECT COUNT(DISTINCT username) FROM audits_userloginlog " +
        "WHERE status=1 AND datetime > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthLoginCount"] = t.getOne(query)
    // 近三月登录资产数
    query = "SELECT COUNT(*) FROM terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthConnectAssetCount"] = t.getOne(query)
    // 近三月文件上传数
    query = "SELECT COUNT(*) FROM audits_ftplog WHERE operate='Upload' " +
        "AND date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthUploadCount"] = t.getOne(query)
    // 近三月命令记录数
    query = "SELECT COUNT(*) FROM terminal_command WHERE " +
        "FROM_UNIXTIME(timestamp) > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthCommandCount"] = t.getOne(query)
    // 近三月高危命令记录数
    query = "SELECT count(*) FROM terminal_command WHERE risk_level=5 and " +
        "FROM_UNIXTIME(timestamp) > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthDangerCommandCount"] = t.getOne(query)
    // 近三月最大会话时长
    query = "SELECT timediff(date_end, date_start) AS duration from terminal_session " +
        "WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH) " +
        "ORDER BY duration DESC LIMIT 1"
    t.result["Last3MonthMaxSessionDuration"] = t.getOne(query)
    // 近三月平均会话时长
    query = "SELECT ROUND(AVG(TIME_TO_SEC(TIMEDIFF(date_end, date_start))), 0) AS duration " +
        "FROM terminal_session WHERE date_start > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthAvgSessionDuration"] = t.getOne(query)
    // 近三月工单申请数
    query = "SELECT COUNT(*) FROM tickets_ticket WHERE date_created > DATE_SUB(CURDATE(), INTERVAL 3 MONTH)"
    t.result["Last3MonthTicketCount"] = t.getOne(query)
}

func (t *SummaryTask) GetChartData() {
    // 按周用户登录折线图
    query := "SELECT DATE(datetime) AS d, COUNT(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= datetime GROUP BY d"
    t.result["UserLoginChart"] = t.getChartCoordinate(query)

    // 按周资产登录折线图
    query = "SELECT DATE(date_start) AS d, COUNT(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 6 DAY) <= date_start GROUP BY d"
    t.result["AssetConnectChart"] = t.getChartCoordinate(query)

    // 月活跃用户柱状图
    query = "SELECT username, count(*) AS num FROM audits_userloginlog " +
        "WHERE status=1 and DATE_SUB(CURDATE(), INTERVAL 1 MONTH) <= datetime " +
        "GROUP BY username ORDER BY num DESC LIMIT 5;"
    t.result["ActiveUserChart"] = t.getChartCoordinate(query)

    // 近3个月活跃资产柱状图
    query = "SELECT asset, count(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
        "GROUP BY asset ORDER BY num DESC LIMIT 5;"
    t.result["ActiveAssetChart"] = t.getChartCoordinate(query)

    // 近3个月各种协议访问饼状图
    var protocolInfos []PieItem
    query = "SELECT protocol, count(*) AS num FROM terminal_session " +
        "WHERE DATE_SUB(CURDATE(), INTERVAL 3 MONTH) <= date_start " +
        "GROUP BY protocol ORDER BY num DESC"
    rows, err := t.client.Query(query)
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
        t.result["ProtocolChart"] = string(data)
    } else {
        t.result["ProtocolChart"] = "[]"
    }
}

func (t *SummaryTask) GetName() string {
    return "信息摘要"
}

func (t *SummaryTask) Run() error {
    t.GetJMSSummary()
    t.GetChartData()
    return nil
}
