package builtin

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"path"
	"strings"

	"github.com/extrame/xls"
	"github.com/xuri/excelize/v2"
)

// ExtractExcelText 从 CSV/XLSX 字节中提取纯文本。
func ExtractExcelText(data []byte, mimeType string, fileName string) string {
	if strings.EqualFold(strings.TrimSpace(mimeType), "text/csv") || strings.EqualFold(strings.TrimPrefix(path.Ext(strings.ToLower(fileName)), "."), "csv") {
		return extractCSVText(data)
	}
	if strings.EqualFold(strings.TrimPrefix(path.Ext(strings.ToLower(fileName)), "."), "xls") {
		return extractXLSText(data)
	}
	return extractXLSXText(data)
}

func extractCSVText(data []byte) string {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	var builder strings.Builder
	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return strings.TrimSpace(string(data))
		}
		if line := normalizeExcelRow(row); line != "" {
			appendExcelLine(&builder, line)
		}
	}
	return strings.TrimSpace(builder.String())
}

func extractXLSXText(data []byte) string {
	workbook, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	defer func() {
		_ = workbook.Close()
	}()

	var builder strings.Builder
	firstSheet := true
	for _, sheetName := range workbook.GetSheetList() {
		rows, rowErr := workbook.Rows(sheetName)
		if rowErr != nil || rows == nil {
			continue
		}
		sheetStarted := false
		for rows.Next() {
			row, rowErr := rows.Columns()
			if rowErr != nil {
				continue
			}
			if line := normalizeExcelRow(row); line != "" {
				if !sheetStarted {
					appendExcelSheetHeader(&builder, sheetName, firstSheet)
					firstSheet = false
					sheetStarted = true
				}
				appendExcelLine(&builder, line)
			}
		}
		_ = rows.Close()
	}
	return strings.TrimSpace(builder.String())
}

func extractXLSText(data []byte) string {
	workbook, err := xls.OpenReader(bytes.NewReader(data), "utf-8")
	if err != nil || workbook == nil {
		return ""
	}

	var builder strings.Builder
	firstSheet := true
	for sheetIndex := 0; sheetIndex < workbook.NumSheets(); sheetIndex++ {
		sheet := workbook.GetSheet(sheetIndex)
		if sheet == nil {
			continue
		}
		sheetStarted := false
		for rowIndex := 0; rowIndex <= int(sheet.MaxRow); rowIndex++ {
			row := safeXLSRow(sheet, rowIndex)
			if row == nil {
				continue
			}
			firstCol := row.FirstCol()
			lastCol := row.LastCol()
			if lastCol <= firstCol {
				continue
			}
			line := normalizeExcelCells(lastCol-firstCol, func(index int) string {
				return row.Col(firstCol + index)
			})
			if line == "" {
				continue
			}
			if !sheetStarted {
				appendExcelSheetHeader(&builder, sheet.Name, firstSheet)
				firstSheet = false
				sheetStarted = true
			}
			appendExcelLine(&builder, line)
		}
	}
	return strings.TrimSpace(builder.String())
}

func safeXLSRow(sheet *xls.WorkSheet, rowIndex int) (row *xls.Row) {
	defer func() {
		if recover() != nil {
			row = nil
		}
	}()
	return sheet.Row(rowIndex)
}

func normalizeExcelRow(row []string) string {
	if len(row) == 0 {
		return ""
	}
	return normalizeExcelCells(len(row), func(index int) string {
		return row[index]
	})
}

func normalizeExcelCells(count int, cellAt func(int) string) string {
	lastNonEmpty := -1
	for idx := 0; idx < count; idx++ {
		if strings.TrimSpace(cellAt(idx)) != "" {
			lastNonEmpty = idx
		}
	}
	if lastNonEmpty < 0 {
		return ""
	}

	var builder strings.Builder
	for idx := 0; idx <= lastNonEmpty; idx++ {
		if idx > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strings.TrimSpace(cellAt(idx)))
	}
	return builder.String()
}

func appendExcelSheetHeader(builder *strings.Builder, sheetName string, firstSheet bool) {
	if !firstSheet {
		builder.WriteString("\n\n")
	}
	builder.WriteString("[Sheet: ")
	builder.WriteString(sheetName)
	builder.WriteString("]")
}

func appendExcelLine(builder *strings.Builder, line string) {
	if builder.Len() > 0 {
		builder.WriteByte('\n')
	}
	builder.WriteString(line)
}
