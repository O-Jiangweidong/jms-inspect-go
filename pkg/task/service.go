package task

import (
    "fmt"
    "inspect/pkg/common"
    "path/filepath"
    "sort"
    "strings"
)

const ComputeSpaceCommand = "du %s -sh|awk '{print $1}'"

type Component struct {
    ServiceName   string
    ServicePort   string
    ServiceStatus string
}

type ServiceTask struct {
    Task
    Machine *Machine
}

func (t *ServiceTask) GetReplayPathInfo() {
    volumeDir := t.GetConfig("VOLUME_DIR", "/")
    replayPath := filepath.Join(volumeDir, "core", "data", "media", "replay")
    t.result["replay_path"] = replayPath
    // 总大小
    cmd := "df -h . --output=size| awk '{if (NR > 1) {print $1}}'"
    if result, err := t.Machine.DoCommand(cmd); err == nil {
        t.result["replay_total"] = result
    } else {
        t.result["replay_total"] = common.Empty
    }
    // 已经使用
    cmd = fmt.Sprintf(ComputeSpaceCommand, replayPath)
    if result, err := t.Machine.DoCommand(cmd); err == nil {
        t.result["replay_used"] = result
    } else {
        t.result["replay_used"] = common.Empty
    }
    // 未使用
    cmd = fmt.Sprintf("cd %s;df -h . --output=avail| awk '{if (NR > 1) {print $1}}'", replayPath)
    if result, err := t.Machine.DoCommand(cmd); err == nil {
        t.result["replay_unused"] = result
    } else {
        t.result["replay_unused"] = common.Empty
    }
}

func (t *ServiceTask) GetComponentLogSize() {
    volumeDir := t.GetConfig("VOLUME_DIR", "/")
    components := []string{
        "nginx", "core", "koko", "lion", "chen", "kael", "magnus",
        "panda", "razor", "video", "xrdp",
    }
    for _, name := range components {
        var logPath string
        version := t.GetConfig("CURRENT_VERSION", "v3")
        if name == "core" && strings.HasPrefix(version, "v2") {
            logPath = filepath.Join(volumeDir, name, "logs")
        } else {
            logPath = filepath.Join(volumeDir, name, "data", "logs")
        }
        cmd := fmt.Sprintf(ComputeSpaceCommand, logPath)
        if result, err := t.Machine.DoCommand(cmd); err == nil {
            t.result[name+"_log_size"] = result
        } else {
            t.result[name+"_log_size"] = common.Empty
        }
    }
}

func (t *ServiceTask) GetJMSServiceStatus() {
    sep := "***"
    var components []Component
    cmd := fmt.Sprintf(`docker ps --format "table {{.Names}}%s{{.Status}}%s{{.Ports}}" |grep jms_`, sep, sep)
    if result, err := t.Machine.DoCommand(cmd); err != nil {
        components = append(components, Component{
            ServiceName: common.Empty, ServicePort: common.Empty,
            ServiceStatus: common.Empty,
        })
    } else {
        lines := strings.Split(result, "\n")[2:]
        for _, line := range lines {
            ret := strings.Split(line, sep)
            portList := strings.Split(strings.Replace(ret[2], " ", "", -1), ",")
            sort.Strings(portList)
            port := strings.Join(portList, "\n")
            components = append(components, Component{
                ServiceName: ret[0], ServiceStatus: ret[1], ServicePort: port,
            })
        }
    }
    t.result["component_info"] = components
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
