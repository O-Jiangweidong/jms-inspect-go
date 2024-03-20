package common

import (
    "bufio"
    "fmt"
    "os"
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

func CurrentDatetime(file bool) string {
    var currentDisplay string
    current := time.Now()
    if file {
        currentDisplay = current.Format("2006-01-02-15-04-05")
    } else {
        currentDisplay = current.Format("2006-01-02 15:04:05")
    }
    return currentDisplay
}
