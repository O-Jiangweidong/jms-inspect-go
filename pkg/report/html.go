package report

import (
    "encoding/json"
    "inspect/pkg/task"
    "io"
    "os"
    "path"
    "strconv"
    "text/template"
)

type PageManager struct {
    num int
}

func (p *PageManager) GetPage() string {
    p.num += 1
    return strconv.Itoa(p.num)
}

func (p *PageManager) CalcPage(i int) string {
    return strconv.Itoa(i*3 + 2)
}

type HtmlReport struct {
    BaseReport

    Summary *task.ResultSummary
}

func Add(num1, num2 int) int {
    return num1 + num2
}

func Json(body any) string {
    if data, err := json.Marshal(body); err != nil {
        return string(data)
    } else {
        return err.Error()
    }
}

func (r *HtmlReport) Generate() error {
    pager := PageManager{num: 0}
    current, err := os.Getwd()
    if err != nil {
        return err
    }
    templatePath := path.Join(current, "templates", "jumpserver_report.html")
    content, err := os.ReadFile(templatePath)
    if err != nil {
        return err
    }

    t, err := template.New("Template").Funcs(
        template.FuncMap{
            "Add": Add, "Json": Json, "GetPage": pager.GetPage,
            "CalcPage": pager.CalcPage,
        },
    ).Parse(string(content))
    if err != nil {
        return err
    }

    outputFile, err := r.GetReportFile("html")
    if err != nil {
        return err
    }
    defer func(outputFile *os.File) {
        _ = outputFile.Close()
    }(outputFile)

    echartsFile, err := os.Open(path.Join(current, "templates", "echarts.min.js"))
    if err != nil {
        return err
    }
    defer func(echartsFile *os.File) {
        _ = echartsFile.Close()
    }(echartsFile)

    echartsData, err := io.ReadAll(echartsFile)
    if err != nil {
        return err
    }
    r.Summary.EchartsData = string(echartsData)
    err = t.Execute(outputFile, r.Summary)
    if err != nil {
        return err
    }
    return nil
}
