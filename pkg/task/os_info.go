package task

import (
	"fmt"
	"inspect/pkg/common"
	"sort"
	"strconv"
	"strings"
)

type OsInfoTask struct {
	Task
	Machine *Machine
}

func (t *OsInfoTask) GetHostname() {
	if result, err := t.Machine.DoCommand("hostname"); err == nil {
		t.result["MachineHostname"] = result
	} else {
		t.result["MachineHostname"] = common.Empty
	}
}

func (t *OsInfoTask) GetLanguage() {
	if result, err := t.Machine.DoCommand("echo $LANG"); err == nil {
		t.result["MachineLanguage"] = result
	} else {
		t.result["MachineLanguage"] = common.Empty
	}
}

func (t *OsInfoTask) GetAllIps() {
	if result, err := t.Machine.DoCommand("hostname -I"); err == nil {
		t.result["MachineAddress"] = result
	} else {
		t.result["MachineAddress"] = common.Empty
	}
}

func (t *OsInfoTask) GetOsVersion() {
	if result, err := t.Machine.DoCommand("uname -o"); err == nil {
		t.result["OsVersion"] = result
	} else {
		t.result["OsVersion"] = common.Empty
	}
}

func (t *OsInfoTask) GetKernelVersion() {
	if result, err := t.Machine.DoCommand("uname -r"); err == nil {
		t.result["KernelVersion"] = result
	} else {
		t.result["KernelVersion"] = common.Empty
	}
}

func (t *OsInfoTask) GetCpuArch() {
	if result, err := t.Machine.DoCommand("uname -m"); err == nil {
		t.result["CpuArch"] = result
	} else {
		t.result["CpuArch"] = common.Empty
	}
}

func (t *OsInfoTask) GetCurrentDatetime() {
	if result, err := t.Machine.DoCommand("date +'%F %T'"); err == nil {
		t.result["CurrentTime"] = result
	} else {
		t.result["CurrentTime"] = common.Empty
	}
}

func (t *OsInfoTask) GetLastUpTime() {
	if result, err := t.Machine.DoCommand("who -b | awk '{print $2,$3,$4}'"); err == nil {
		t.result["LastUpTime"] = result
	} else {
		t.result["LastUpTime"] = common.Empty
	}
}

func (t *OsInfoTask) GetOperatingTime() {
	if result, err := t.Machine.DoCommand("cat /proc/uptime | awk '{print $1}'"); err == nil {
		if seconds, err := strconv.Atoi(strings.Split(result, ".")[0]); err == nil {
			t.result["OperatingTime"] = common.SecondDisplay(seconds)
			return
		}
	}
	t.result["OperatingTime"] = common.Empty
}

func (t *OsInfoTask) GetCPUInfo() {
	// CPU 数
	coreNumCmd := `cat /proc/cpuinfo | grep "physical id" | sort | uniq | wc -l`
	if result, err := t.Machine.DoCommand(coreNumCmd); err == nil {
		t.result["CpuNum"] = result
	} else {
		t.result["CpuNum"] = common.Empty
	}
	// 每物理核心数
	physicalCmd := `cat /proc/cpuinfo | grep "cpu cores" | uniq`
	if result, err := t.Machine.DoCommand(physicalCmd); err == nil {
		t.result["CpuPhysicalCores"] = result
	} else {
		t.result["CpuPhysicalCores"] = common.Empty
	}
	// 逻辑核数
	logicalCmd := `cat /proc/cpuinfo | grep "processor" | wc -l`
	if result, err := t.Machine.DoCommand(logicalCmd); err == nil {
		t.result["CpuLogicalCores"] = result
	} else {
		t.result["CpuLogicalCores"] = common.Empty
	}
	// CPU 型号
	cpuModelCmd := `cat /proc/cpuinfo | grep name | cut -f2 -d: | uniq`
	if result, err := t.Machine.DoCommand(cpuModelCmd); err == nil {
		t.result["CpuModel"] = result
	} else {
		t.result["CpuModel"] = common.Empty
	}
}

func (t *OsInfoTask) GetMemoryInfo() {
	// 物理内存信息
	physicalCmd := `free -h|grep -i mem`
	if result, err := t.Machine.DoCommand(physicalCmd); err == nil {
		var resultList []string
		tempList := strings.Split(result, " ")
		for _, item := range tempList[1:] {
			if item != "" {
				resultList = append(resultList, item)
			}
		}
		t.result["MemoryTotal"] = resultList[0]
		t.result["MemoryUsed"] = resultList[1]
		t.result["MemoryAvailable"] = resultList[len(resultList)-1]
	} else {
		t.result["MemoryTotal"] = common.Empty
		t.result["MemoryUsed"] = common.Empty
		t.result["MemoryAvailable"] = common.Empty
	}
	// 虚拟内存信息
	virtualCmd := `free -h|grep -i swap`
	if result, err := t.Machine.DoCommand(virtualCmd); err == nil {
		var resultList []string
		tempList := strings.Split(result, " ")
		for _, item := range tempList[1:] {
			if item != "" {
				resultList = append(resultList, item)
			}
		}
		t.result["SwapTotal"] = resultList[0]
		t.result["SwapUsed"] = resultList[1]
		t.result["SwapFree"] = resultList[2]
	} else {
		t.result["SwapTotal"] = common.Empty
		t.result["SwapUsed"] = common.Empty
		t.result["SwapFree"] = common.Empty
	}
}

type DiskInfo struct {
	FileSystem    string
	FileType      string
	FileSize      string
	FileUsed      string
	FileAvailable string
	FileUsageRate string
	FileMount     string
}

func (t *OsInfoTask) GetDiskInfo() {
	logicalCmd := `df -hT -x tmpfs -x overlay -x devtmpfs| awk '{if (NR > 1 && $1!=tmpfs) {print $1,$2,$3,$4,$5,$6,$7}}'`
	var diskInfoList []DiskInfo
	if result, err := t.Machine.DoCommand(logicalCmd); err == nil {
		for _, disk := range strings.Split(result, "\n") {
			if disk == "" {
				continue
			}
			diskInfo := strings.Split(disk, " ")
			diskInfoList = append(diskInfoList, DiskInfo{
				FileSystem:    strings.TrimSpace(diskInfo[0]),
				FileType:      strings.TrimSpace(diskInfo[1]),
				FileSize:      strings.TrimSpace(diskInfo[2]),
				FileUsed:      strings.TrimSpace(diskInfo[3]),
				FileAvailable: strings.TrimSpace(diskInfo[4]),
				FileUsageRate: strings.TrimSpace(diskInfo[5]),
				FileMount:     strings.TrimSpace(diskInfo[6]),
			})
		}
		t.result["DiskInfoList"] = diskInfoList
	} else {
		t.result["DiskInfoList"] = diskInfoList
	}
}

func (t *OsInfoTask) GetSystemParams() {
	// SELinux是否开启
	if result, err := t.Machine.DoCommand("getenforce"); err == nil {
		t.result["SelinuxEnable"] = result
	} else {
		t.result["SelinuxEnable"] = common.Empty
	}
	// 防火墙是否开启
	firewalldCmd := `systemctl status firewalld | grep active > /dev/null 2>&1;if [[ $? -eq 0 ]]; then echo 1;else echo 0;fi`
	if result, err := t.Machine.DoCommand(firewalldCmd); err == nil {
		enable := common.BoolDisplay(result)
		t.result["FirewallEnable"] = enable
		if enable == common.No {
			t.SetAbnormalEvent("节点下防火墙未开启", common.Critical)
		}
	} else {
		t.result["FirewallEnable"] = common.Empty
	}
	// 是否开启 RSyslog
	syslogCmd := `systemctl status rsyslog | grep active > /dev/null 2>&1;if [[ $? -eq 0 ]];then echo 1;else echo 0;fi`
	if result, err := t.Machine.DoCommand(syslogCmd); err == nil {
		t.result["RsyslogEnable"] = common.BoolDisplay(result)
	} else {
		t.result["RsyslogEnable"] = common.Empty
	}
	// 是否存在定时任务
	if result, err := t.Machine.DoCommand("ls /var/spool/cron/ |wc -l"); err == nil {
		t.result["CrontabEnable"] = common.BoolDisplay(result)
	} else {
		t.result["CrontabEnable"] = common.Empty
	}
}

func (t *OsInfoTask) GetPortTidyDisplay(result string) string {
	portList := []int{99999}
	tempPortList := strings.Split(result, ",")
	for _, port := range tempPortList {
		if p, err := strconv.Atoi(port); err != nil {
			continue
		} else {
			portList = append(portList, p)
		}
	}
	sort.Ints(portList)
	var finallyPort []string
	start := ""
	for i := 0; i < len(portList)-1; i++ {
		portStr := strconv.Itoa(portList[i])
		if portList[i]+1 == portList[i+1] {
			if start == "" {
				start = portStr
			}
		} else if start != "" {
			finallyPort = append(finallyPort, fmt.Sprintf("%s-%s", start, portStr))
			start = ""
		} else {
			finallyPort = append(finallyPort, portStr)
		}
	}
	return strings.Join(finallyPort, ", ")
}

func (t *OsInfoTask) GetExposePort() {
	ssCmd := `ss -tuln | grep LISTEN | awk '{print $5}' | awk -F: '{print $2$4}' | sort |uniq |tr '\n' ',' | sed 's/,$//'`
	if result, err := t.Machine.DoCommand(ssCmd); err == nil {
		ports := t.GetPortTidyDisplay(result)
		t.result["ExposePort"] = ports
	} else {
		t.result["ExposePort"] = common.Empty
	}
}

func (t *OsInfoTask) GetZombieProcess() {
	if result, err := t.Machine.DoCommand("ps -e -o ppid,stat | grep Z| wc -l"); err == nil {
		exist := common.BoolDisplay(result)
		t.result["ExistZombie"] = exist
		if exist == common.Yes {
			t.SetAbnormalEvent("节点下存在僵尸进程", common.NORMAL)
		}
	} else {
		t.result["ExistZombie"] = common.Empty
	}
}

func (t *OsInfoTask) GetName() string {
	return "机器当前系统检查"
}

func (t *OsInfoTask) Run() error {
	t.GetHostname()
	t.GetLanguage()
	t.GetAllIps()
	t.GetOsVersion()
	t.GetKernelVersion()
	t.GetCpuArch()
	t.GetCurrentDatetime()
	t.GetLastUpTime()
	t.GetOperatingTime()
	t.GetCPUInfo()
	t.GetMemoryInfo()
	t.GetDiskInfo()
	t.GetSystemParams()
	t.GetExposePort()
	t.GetZombieProcess()
	return nil
}
