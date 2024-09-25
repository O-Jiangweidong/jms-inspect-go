package report

import (
	_ "embed"
	"encoding/json"
	"os"
	"strconv"
	"text/template"

	"inspect/pkg/task"
)

//go:embed templates/jumpserver_report.html
var templateData string

//go:embed templates/echarts.min.js
var echartsData string

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

	t, err := template.New("Template").Funcs(
		template.FuncMap{
			"Add": Add, "Json": Json, "GetPage": pager.GetPage,
			"CalcPage": pager.CalcPage,
		},
	).Parse(templateData)
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
	r.Summary.EchartsData = echartsData
	err = t.Execute(outputFile, r.Summary)
	if err != nil {
		return err
	}
	return nil
}
