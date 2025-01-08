package report

import (
	"encoding/json"
	"errors"
	"fmt"
	"inspect/pkg/common"
	"inspect/pkg/task"
	"os"

	"github.com/tealeg/xlsx"
)

type ExcelReport struct {
	BaseReport

	Summary *task.ResultSummary
}

func (r *ExcelReport) WriteSheet(file *xlsx.File, sheetName string, data interface{}) error {
	sheet, err := file.AddSheet(sheetName)
	if err != nil {
		return err
	}
	switch val := data.(type) {
	case map[string]interface{}:
		data = val
	default:
		return errors.New("unsupported data type")
	}
	for key, value := range data.(map[string]interface{}) {
		var headCell *xlsx.Cell
		row := sheet.AddRow()
		switch val := value.(type) {
		case []interface{}:
			needMergeLength := 0
			headCell = row.AddCell()
			headCell.SetString(key)

			for i, v1 := range val {
				for k, v2 := range v1.(map[string]interface{}) {
					needMergeLength += 1
					if needMergeLength > 1 {
						row = sheet.AddRow()
						row.AddCell().Value = ""
					}
					row.AddCell().Value = k
					row.AddCell().Value = fmt.Sprint(v2)
				}
				if i != len(val)-1 {
					needMergeLength += 1
					row = sheet.AddRow()
				}
			}
			headCell.Merge(0, needMergeLength-1)
		case map[string]interface{}:
			needMergeLength := 0
			headCell = row.AddCell()
			headCell.SetString(key)
			for k, v := range val {
				needMergeLength += 1
				if needMergeLength > 1 {
					row = sheet.AddRow()
					row.AddCell().Value = ""
				}
				row.AddCell().Value = k
				row.AddCell().Value = fmt.Sprint(v)
			}
			headCell.Merge(0, needMergeLength-1)
		default:
			row.AddCell().Value = key
			row.AddCell().Value = ""
			if val == nil {
				val = common.Empty
			}
			row.AddCell().Value = fmt.Sprint(val)
		}
	}

	for colIndex := 0; colIndex < len(sheet.Rows[0].Cells); colIndex++ {
		maxWidth := 0
		for _, row := range sheet.Rows {
			if len(row.Cells) > colIndex {
				cell := row.Cells[colIndex]
				width := len([]rune(cell.String()))
				if width > maxWidth {
					maxWidth = width
				}
			}
		}
		sheet.Cols[colIndex].Width = float64(maxWidth)
	}
	style := xlsx.NewStyle()
	style.Alignment.Horizontal = "center"
	style.Alignment.Vertical = "center"
	sheet.Cols[0].SetStyle(style)
	return nil
}

func (r *ExcelReport) Generate() error {
	outputFile, err := r.GetReportFile("xlsx")
	if err != nil {
		return err
	}

	defer func(outputFile *os.File) {
		_ = outputFile.Close()
	}(outputFile)

	file := xlsx.NewFile()

	var data map[string]interface{}
	jsonData, err := json.Marshal(r.Summary)
	err = json.Unmarshal(jsonData, &data)
	for name, value := range data {
		if value == nil {
			continue
		}
		switch val := value.(type) {
		case []interface{}:
			for i, v1 := range val {
				err = r.WriteSheet(file, fmt.Sprintf("%s-%v", name, i), v1)
			}
		default:
			err = r.WriteSheet(file, name, val)
		}
	}
	if err != nil {
		return err
	}
	_ = file.Write(outputFile)
	return nil
}
