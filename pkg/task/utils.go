package task

import (
	"inspect/pkg/common"
	"strconv"
	"time"
)

func DoTask(task AbstractTask, opts *Options) (map[string]interface{}, []AbnormalMsg) {
	logger := common.GetLogger()
	start := time.Now()
	err := task.Init(opts)
	if err != nil {
		logger.Error("初始化任务失败: %s", err)
	}
	logger.StartTip("正在执行任务：%s", task.GetName())
	err = task.Run()
	duration := strconv.FormatFloat(time.Now().Sub(start).Seconds(), 'f', 2, 64)
	logger.StopTip("[成功]:> 执行任务：%s（耗时：%s秒）", task.GetName(), duration)
	if err != nil {
		logger.Warning("执行任务出错: %s", err)
	}
	return task.GetResult()
}
