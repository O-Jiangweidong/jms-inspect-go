package main

import (
    "flag"

    _ "github.com/go-sql-driver/mysql"

    "inspect/pkg/common"
    "inspect/pkg/report"
    "inspect/pkg/task"
)

const DefaultJMSConfigPath = "/opt/jumpserver/config/config.txt"

var logger *common.Logger

func main() {
    logger = common.NewLogger()
    opts := task.Options{Logger: logger}
    defer opts.Clear()

    flag.StringVar(
        &opts.ReportType, "t", "html",
        "生成报告的类型(当前支持html)",
    )
    flag.StringVar(
        &opts.JMSConfigPath, "jc", DefaultJMSConfigPath, "堡垒机配置文件路径",
    )
    flag.StringVar(
        &opts.MachineInfoPath, "mt", opts.MachineInfoPath,
        "待巡检机器配置文件路径(查看脚本压缩包内 demo.csv 文件)",
    )
    flag.StringVar(
        &opts.ExcludeTask, "et", opts.ExcludeTask,
        "不执行的任务，多个任务中间用逗号隔开(mysql、redis)",
    )
    flag.Parse()

    logger.Debug("开始检查配置等相关信息...")
    if err := opts.Valid(); err != nil {
        logger.Error("参数校验错误: %v\n", err)
    }

    var resultSummary task.ResultSummary
    var result map[string]interface{}
    var abnormalResult []task.AbnormalMsg
    logger.Info("巡检任务开始")
    // 设置全局信息
    resultSummary.SetGlobalInfo(&opts)
    // 执行摘要任务
    summaryTask := task.SummaryTask{}
    result, _ = task.DoTask(&summaryTask, &opts)
    resultSummary.VirtualResult = result

    // 执行组件依赖任务
    dbTask := task.DBTask{}
    result, _ = task.DoTask(&dbTask, &opts)
    resultSummary.DBResult = result

    var resultList []map[string]interface{}
    for _, m := range opts.MachineSet {
        executor := m.GetExecutor()
        executor.Logger = logger
        result, abnormalResult = executor.Execute(&opts)
        result["MachineType"] = m.Type
        result["MachineName"] = m.Name
        resultList = append(resultList, result)
        for _, r := range abnormalResult {
            r.NodeName = m.Name
            resultSummary.AbnormalResults = append(resultSummary.AbnormalResults, r)
        }
    }
    resultSummary.NormalResults = resultList

    r := report.HtmlReport{Summary: &resultSummary}
    if err := r.Generate(); err != nil {
        logger.Error("生成报告错误: %s", err)
    }
    logger.Info("巡检完成，请将此路径下的巡检文件发送给技术工程师: \n%s", r.ReportPath)
}
