package builtin

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
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

func TestExcelizeRejectsInvalidRowIndex(t *testing.T) {
	file := excelize.NewFile()
	defer func() {
		_ = file.Close()
	}()
	if err := file.SetCellValue("Sheet1", "A1", "value"); err != nil {
		t.Fatal(err)
	}

	var source bytes.Buffer
	if err := file.Write(&source); err != nil {
		t.Fatal(err)
	}
	malformed := rewriteXLSXWorksheet(t, source.Bytes(), func(data []byte) []byte {
		updated := bytes.Replace(data, []byte(`<row r="1"`), []byte(`<row r="-1"`), 1)
		if bytes.Equal(updated, data) {
			t.Fatal("expected worksheet row index to be replaced")
		}
		return updated
	})

	workbook, err := excelize.OpenReader(bytes.NewReader(malformed))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = workbook.Close()
	}()
	if _, err = workbook.GetCellValue("Sheet1", "A1"); err == nil {
		t.Fatal("expected invalid worksheet row index to be rejected")
	}
}

func rewriteXLSXWorksheet(t *testing.T, data []byte, rewrite func([]byte) []byte) []byte {
	t.Helper()

	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	writer := zip.NewWriter(&output)
	for _, item := range reader.File {
		entryReader, openErr := item.Open()
		if openErr != nil {
			t.Fatal(openErr)
		}
		entry, readErr := io.ReadAll(entryReader)
		closeErr := entryReader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		if item.Name == "xl/worksheets/sheet1.xml" {
			entry = rewrite(entry)
		}
		entryWriter, createErr := writer.CreateHeader(&zip.FileHeader{
			Name:   item.Name,
			Method: item.Method,
		})
		if createErr != nil {
			t.Fatal(createErr)
		}
		if _, writeErr := entryWriter.Write(entry); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
