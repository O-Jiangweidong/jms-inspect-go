package report

import (
	"fmt"
	"inspect/pkg/common"
	"os"
	"path"
)

type BaseReport struct {
	ReportDir  string
	ReportPath string
}

func (r *BaseReport) GetReportFile(ext string) (*os.File, error) {
	filename := fmt.Sprintf("JumpServer巡检报告_%s.%s", common.CurrentDatetime(true), ext)
	outputDir, err := common.GetOutputDir()
	if err != nil {
		return nil, err
	}
	r.ReportDir = outputDir
	r.ReportPath = path.Join(outputDir, filename)
	outputFile, err := os.Create(r.ReportPath)
	if err != nil {
		return nil, nil
	}
	return outputFile, nil
}
