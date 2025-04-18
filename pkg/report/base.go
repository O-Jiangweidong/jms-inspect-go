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
	filename := fmt.Sprintf("JumpServer巡检报告_%s.%s", common.CurrentDatetime("file"), ext)
	r.ReportDir = common.OutputDir
	r.ReportPath = path.Join(common.OutputDir, filename)
	outputFile, err := os.Create(r.ReportPath)
	if err != nil {
		return nil, nil
	}
	return outputFile, nil
}
