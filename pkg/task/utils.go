package task

import (
    "inspect/pkg/common"
    "strconv"
    "time"
)

func DoTask(task AbstractTask, opts *Options) (map[string]interface{}, []AbnormalMsg) {
    logger := common.NewLogger()
    start := time.Now()
    err := task.Init(opts)
    if err != nil {
        logger.Error("初始化任务失败: %s", err)
    }
    logger.Info("开始执行任务：%s", task.GetName())
    err = task.Run()
    if err != nil {
        logger.Warning("执行任务出错: %s", err)
    }
    duration := strconv.FormatFloat(time.Now().Sub(start).Seconds(), 'f', 2, 64)
    logger.Info("执行结束（耗时：%s秒）\n", duration)
    return task.GetResult()
}
