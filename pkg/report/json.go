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
    jsonData, err := json.Marshal(r.Summary)
    if err != nil {
        return err
    }

    outputFile, err := r.GetReportFile("json")
    if err != nil {
        return err
    }
    defer func(outputFile *os.File) {
        _ = outputFile.Close()
    }(outputFile)

    _, err = outputFile.Write(jsonData)
    if err != nil {
        return err
    }
    return nil
}
