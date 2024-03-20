package report

import (
    "fmt"
    "inspect/pkg/common"
    "os"
    "path"
)

type BaseReport struct {
    ReportPath string
}

func (r *BaseReport) GetReportFile(ext string) (*os.File, error) {
    current, err := os.Getwd()
    if err != nil {
        return nil, err
    }
    filename := fmt.Sprintf("JumpServer巡检报告-%s.%s", common.CurrentDatetime(true), ext)
    outputDir := path.Join(current, "output")
    if file, err := os.Stat(outputDir); err != nil || !file.IsDir() {
        if err = os.Mkdir(outputDir, 0700); err != nil {
            return nil, err
        }
    }
    r.ReportPath = path.Join(outputDir, filename)
    outputFile, err := os.Create(r.ReportPath)
    if err != nil {
        return nil, nil
    }
    return outputFile, nil
}
