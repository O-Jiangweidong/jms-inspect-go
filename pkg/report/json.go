package report

import (
	"encoding/json"
	"inspect/pkg/task"
	"os"
)

type JsonReport struct {
	BaseReport

	Summary *task.ResultSummary
}

func (r *JsonReport) Generate() error {
	outputFile, err := r.GetReportFile("json")
	if err != nil {
		return err
	}
	defer func(outputFile *os.File) {
		_ = outputFile.Close()
	}(outputFile)

	encoder := json.NewEncoder(outputFile)
	encoder.SetEscapeHTML(false)
	if err = encoder.Encode(r.Summary); err != nil {
		return err
	}
	return nil
}
