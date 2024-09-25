package common

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"time"
)

func ConfigFileToMap(filepath string) (map[string]string, error) {
	configMap := make(map[string]string)
	file, err := os.Open(filepath)
	if err != nil {
		return nil, fmt.Errorf("打开文件[%s]失败，错误: %v", filepath, err)
	}
	defer func(file *os.File) {
		_ = file.Close()
	}(file)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		value := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(value, "#") {
			continue
		}
		items := strings.SplitN(value, "=", 2)
		if len(items) != 2 {
			continue
		}
		configMap[items[0]] = items[1]
	}
	return configMap, nil
}

func BoolDisplay(value string) string {
	if res, _ := strconv.ParseBool(value); res {
		return Yes
	} else {
		return No
	}
}

func SecondDisplay(second int) string {
	display := ""
	duration := time.Duration(second) * time.Second
	hours := duration / time.Hour
	if hours != 0 {
		display += fmt.Sprintf("%d 小时", hours)
	}
	duration = duration % time.Hour
	minutes := duration / time.Minute
	if minutes != 0 {
		display += fmt.Sprintf("%d 分钟", minutes)
	}
	duration = duration % time.Minute
	seconds := duration / time.Second
	if seconds != 0 {
		display += fmt.Sprintf("%d 秒", minutes)
	}
	return display
}

func CurrentDatetime(needType string) string {
	var currentDisplay string
	current := time.Now()
	switch needType {
	case "file":
		currentDisplay = current.Format("20060102_150405")
	case "dir":
		currentDisplay = current.Format("2006010215044")
	default:
		currentDisplay = current.Format("2006-01-02 15:04:05")
	}
	return currentDisplay
}

func SpaceDisplay(sizeKB int64) string {
	const (
		KB = 1024
		MB = 1 * KB
		GB = MB * KB
		TB = GB * KB
		PB = TB * KB
	)
	switch {
	case sizeKB >= PB:
		return fmt.Sprintf("%.1fPB", float64(sizeKB)/float64(PB))
	case sizeKB >= TB:
		return fmt.Sprintf("%.1fTB", float64(sizeKB)/float64(TB))
	case sizeKB >= GB:
		return fmt.Sprintf("%.1fGB", float64(sizeKB)/float64(GB))
	case sizeKB >= MB:
		return fmt.Sprintf("%.1fMB", float64(sizeKB)/float64(MB))
	default:
		return fmt.Sprintf("%dKB", sizeKB)
	}
}

func GetOutputDir() (string, error) {
	current, err := os.Getwd()
	if err != nil {
		return "", err
	}
	outputDir := path.Join(current, "output", CurrentDatetime("dir"))
	if file, err := os.Stat(outputDir); err != nil || !file.IsDir() {
		if err = os.Mkdir(outputDir, 0700); err != nil {
			return "", err
		}
	}
	return outputDir, nil
}
