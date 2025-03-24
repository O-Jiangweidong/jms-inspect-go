package task

import (
	"fmt"
	"inspect/pkg/common"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const ComputeSpaceCommand = "[ -d %s ] && du %s -sh|awk '{print $1}' || echo '%s'"

type Component struct {
	ServiceName    string
	ServicePort    string
	ServiceStatus  string
	ServiceLogSize string
}

type ServiceTask struct {
	Task
	Machine *Machine
}

func (t *ServiceTask) GetReplayPathInfo() {
	volumeDir := t.GetConfig("VOLUME_DIR", "/")
	replayPath := filepath.Join(volumeDir, "core", "data", "media", "replay")
	t.result["ReplayPath"] = replayPath
	// 总大小
	cmd := fmt.Sprintf(
		"[ -d %s ] && df -h %s --output=size| awk '{if (NR > 1) {print $1}}' || echo '0'",
		replayPath, replayPath,
	)
	command := Command{content: cmd, timeout: 5}
	if result, err := t.Machine.DoCommand(command); err == nil && result != common.EmptyFlag {
		t.result["ReplayTotal"] = result
	} else {
		t.result["ReplayTotal"] = common.Empty
	}
	// 已经使用
	cmd = fmt.Sprintf(ComputeSpaceCommand, replayPath, replayPath, common.EmptyFlag)
	command = Command{content: cmd, timeout: 5}
	if result, err := t.Machine.DoCommand(command); err == nil && result != common.EmptyFlag {
		t.result["ReplayUsed"] = result
	} else {
		t.result["ReplayUsed"] = common.Empty
	}
	// 未使用
	cmd = fmt.Sprintf(
		"[ -d %s ] && df %s --output=avail| awk '{if (NR > 1) {print $1}}' || echo '%s'",
		replayPath, replayPath, common.EmptyFlag,
	)
	command = Command{content: cmd, timeout: 5}
	if result, err := t.Machine.DoCommand(command); err == nil && result != common.EmptyFlag {
		if size, err := strconv.ParseInt(result, 10, 64); err != nil {
			t.result["ReplayUnused"] = common.Empty
		} else {
			sizeDisplay := common.SpaceDisplay(size)
			t.result["ReplayUnused"] = sizeDisplay
			if size <= 50*1024 {
				desc := fmt.Sprintf("录像空间大小不足，当前大小: %s", sizeDisplay)
				t.SetAbnormalEvent(desc, common.Critical)
			}
		}
	} else {
		t.result["ReplayUnused"] = common.Empty
	}
}

func (t *ServiceTask) GetComponentLogSize() {
	var components []Component
	volumeDir := t.GetConfig("VOLUME_DIR", "/")
	componentNames := []string{
		"Nginx", "Core", "KoKo", "Lion", "Chen", "Kael", "Magnus",
		"Panda", "Razor", "Video", "XRDP", "Facelive", "Nec",
	}
	for _, name := range componentNames {
		var logPath string
		var needRecord bool
		version := t.GetConfig("CURRENT_VERSION", "v3")
		subPath := strings.ToLower(name)
		if subPath == "core" && strings.HasPrefix(version, "v2") {
			logPath = filepath.Join(volumeDir, subPath, "logs")
		} else {
			logPath = filepath.Join(volumeDir, subPath, "data", "logs")
		}
		logSize := common.Empty
		cmd := fmt.Sprintf(ComputeSpaceCommand, logPath, logPath, common.EmptyFlag)
		command := Command{content: cmd, timeout: 5}
		if result, err := t.Machine.DoCommand(command); err == nil {
			if result != common.EmptyFlag {
				needRecord = true
			}
			logSize = result
		}
		if needRecord {
			components = append(components, Component{
				ServiceName:    fmt.Sprintf("%s 日志大小", name),
				ServiceLogSize: logSize,
			})
		}
	}
	t.result["ComponentLogSize"] = components
}

func (t *ServiceTask) GetJMSServiceStatus() {
	sep := "***"
	var components []Component
	cmd := fmt.Sprintf(`docker ps --format "table {{.Names}}%s{{.Status}}%s{{.Ports}}" |grep jms_`, sep, sep)
	command := Command{content: cmd, timeout: 5}
	if result, err := t.Machine.DoCommand(command); err != nil {
		components = append(components, Component{
			ServiceName: common.Empty, ServicePort: common.Empty,
			ServiceStatus: common.Empty,
		})
	} else {
		lines := strings.Split(result, "\n")
		for _, line := range lines {
			ret := strings.Split(line, sep)
			if len(ret) != 3 {
				continue
			}
			portList := strings.Split(strings.Replace(ret[2], " ", "", -1), ",")
			sort.Strings(portList)
			port := strings.Join(portList, "\n")
			components = append(components, Component{
				ServiceName: ret[0], ServiceStatus: ret[1], ServicePort: port,
			})
		}
	}
	t.result["ComponentInfo"] = components
}

func (t *ServiceTask) GetName() string {
	return "堡垒机服务检查"
}

func (t *ServiceTask) Run() error {
	t.GetReplayPathInfo()
	t.GetComponentLogSize()
	t.GetJMSServiceStatus()
	return nil
}
