package task

import (
	"database/sql"
	"fmt"
	"strings"
)

type SummaryTask struct {
	Task

	client RDSClient
}

func (t *SummaryTask) Init(opts *Options) error {
	t.Options = opts
	t.result = make(map[string]interface{})
	client, err := opts.GetRDSClient()
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

func (t *SummaryTask) GetJMSSummary() {
	// 获取用户总数
	query := "SELECT COUNT(*) FROM users_user WHERE is_service_account=false"
	t.result["UserCount"] = t.getOne(query)
	// 获取资产总数
	query = "SELECT COUNT(*) FROM assets_asset"
	t.result["AssetCount"] = t.getOne(query)
	// 获取在线会话总数
	query = "SELECT COUNT(*) FROM terminal_session WHERE is_finished=false"
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
			display = append(display, fmt.Sprintf("%s类型：%s个", platform, count))
		}
	}
	t.result["AssetCountDisplay"] = strings.Join(display, "，")
	// 获取组织数量
	query = "SELECT COUNT(*) FROM orgs_organization"
	t.result["OrganizationCount"] = t.getOne(query)
	// 获取最大单日登录次数
	t.result["MaxLoginCount"] = t.client.GetMaxLoginCount()
	// 最大单日访问资产数
	t.result["MaxLoginAssetCount"] = t.client.GetMaxLoginAssetCount()
	// 近三月最大单日用户登录数
	t.result["Last3MonthMaxLoginCount"] = t.client.GetMaxLoginUsersInLast3Months()
	// 近三月最大单日资产登录数
	t.result["Last3MonthMaxLoginAssetCount"] = t.client.GetMaxAssetLoginsInLast3Months()
	// 近一月登录用户数
	count := t.client.GetUserLoginsInLastXMonths(1)
	t.result["Last1MonthLoginCount"] = count
	// 近一月登录资产数
	count = t.client.GetAssetLoginsInLastXMonths(1)
	t.result["Last1MonthConnectAssetCount"] = count
	// 近一月文件上传数
	count = t.client.GetFTPLogsInLastXMonths(1)
	t.result["Last1MonthUploadCount"] = count
	// 近三月登录用户数
	count = t.client.GetUserLoginsInLastXMonths(3)
	t.result["Last3MonthLoginCount"] = count
	// 近三月登录资产数
	count = t.client.GetAssetLoginsInLastXMonths(3)
	t.result["Last3MonthConnectAssetCount"] = count
	// 近三月文件上传数
	count = t.client.GetFTPLogsInLastXMonths(3)
	t.result["Last3MonthUploadCount"] = count
	// 近三月命令记录数
	count = t.client.GetCommandCountInLastXMonths(3, 0)
	t.result["Last3MonthCommandCount"] = count
	// 近三月高危命令记录数
	count = t.client.GetCommandCountInLastXMonths(3, 5)
	t.result["Last3MonthDangerCommandCount"] = count
	// 近三月最大会话时长
	duration := t.client.GetMaxDurationInLastXMonths(3)
	t.result["Last3MonthMaxSessionDuration"] = duration
	// 近三月平均会话时长
	duration = t.client.GetAvgDurationInLastXMonths(3)
	t.result["Last3MonthAvgSessionDuration"] = duration
	// 近三月工单申请数
	count = t.client.GetTicketCountInLastXMonths(3)
	t.result["Last3MonthTicketCount"] = count
}

func (t *SummaryTask) GetChartData() {
	// 按周用户登录折线图
	t.result["UserLoginChart"] = t.client.GetUserLoginChart()
	// 按周资产登录折线图
	t.result["AssetLoginChart"] = t.client.GetAssetLoginChart()
	// 月活跃用户柱状图
	t.result["ActiveUserChart"] = t.client.GetActiveUserChart()
	// 近3个月活跃资产柱状图
	t.result["ActiveAssetChart"] = t.client.GetActiveAssetChart()
	// 近3个月各种协议访问饼状图
	t.result["ProtocolChart"] = t.client.GetProtocolsAccessPie()
}

func (t *SummaryTask) GetName() string {
	return "信息摘要"
}

func (t *SummaryTask) Run() error {
	t.GetJMSSummary()
	t.GetChartData()
	return nil
}
