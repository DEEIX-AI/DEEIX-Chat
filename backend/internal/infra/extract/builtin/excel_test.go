package builtin

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestExtractCSVTextPreservesRowsAndColumns(t *testing.T) {
	var builder strings.Builder
	const rows = 2105
	const columns = 90
	for row := 0; row < rows; row++ {
		for col := 0; col < columns; col++ {
			if col > 0 {
				builder.WriteString(",")
			}
			builder.WriteString(fmt.Sprintf("r%dc%d", row, col))
		}
		builder.WriteString("\n")
	}

	output := ExtractExcelText([]byte(builder.String()), "text/csv", "data.csv")
	lines := strings.Split(output, "\n")
	if len(lines) != rows {
		t.Fatalf("expected %d rows, got %d", rows, len(lines))
	}
	extractedColumns := strings.Split(lines[0], ",")
	if len(extractedColumns) != columns {
		t.Fatalf("expected %d columns, got %d", columns, len(extractedColumns))
	}
	if !strings.Contains(output, "r2104c89") {
		t.Fatal("expected last row and column to be preserved")
	}
}

func TestExtractXLSXTextPreservesRows(t *testing.T) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()
	const rows = 2105
	for row := 1; row <= rows; row++ {
		cell, err := excelize.CoordinatesToCellName(1, row)
		if err != nil {
			t.Fatal(err)
		}
		if err = file.SetCellValue("Sheet1", cell, fmt.Sprintf("row-%d", row)); err != nil {
			t.Fatal(err)
		}
	}
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		t.Fatal(err)
	}

	output := ExtractExcelText(buffer.Bytes(), "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "data.xlsx")
	if !strings.Contains(output, fmt.Sprintf("row-%d", rows)) {
		t.Fatal("expected last row to be preserved")
	}
}
