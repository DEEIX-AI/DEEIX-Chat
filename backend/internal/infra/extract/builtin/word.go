package builtin

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	docx "github.com/mmonterroca/docxgo/v2"
	"github.com/mmonterroca/docxgo/v2/domain"
)

const (
	wordFormatDOC  = "doc"
	wordFormatDOCX = "docx"
)

type wordCommandRunner interface {
	LookPath(file string) (string, error)
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, err error)
}

type execWordRunner struct{}

type WordResult struct {
	Text   string
	Engine string
}

// ExtractWordText 从 DOC / DOCX 中提取纯文本。
func ExtractWordText(ctx context.Context, absPath string, data []byte, mimeType string, fileName string) WordResult {
	return extractWordText(ctx, absPath, data, mimeType, fileName, execWordRunner{})
}

// ExtractDocxText 从 DOCX 字节中提取纯文本。
func ExtractDocxText(data []byte) string {
	document, err := docx.OpenDocumentFromBytes(data)
	if err != nil {
		return ""
	}
	return normalizeDocumentText(extractDocumentText(document))
}

func extractWordText(ctx context.Context, absPath string, data []byte, mimeType string, fileName string, runner wordCommandRunner) WordResult {
	switch resolveWordFormat(mimeType, fileName) {
	case wordFormatDOC:
		return WordResult{
			Text:   extractDocText(ctx, absPath, runner),
			Engine: "builtin_doc",
		}
	default:
		return WordResult{
			Text:   ExtractDocxText(data),
			Engine: "builtin_docx",
		}
	}
}

func resolveWordFormat(mimeType string, fileName string) string {
	mime := strings.ToLower(strings.TrimSpace(mimeType))
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(strings.TrimSpace(fileName)), "."))
	switch {
	case ext == wordFormatDOC:
		return wordFormatDOC
	case ext == wordFormatDOCX:
		return wordFormatDOCX
	case strings.Contains(mime, "wordprocessingml"):
		return wordFormatDOCX
	case strings.Contains(mime, "msword"):
		return wordFormatDOC
	default:
		return wordFormatDOCX
	}
}

func extractDocText(ctx context.Context, absPath string, runner wordCommandRunner) string {
	if strings.TrimSpace(absPath) == "" || runner == nil {
		return ""
	}
	extractors := []func(context.Context, string, wordCommandRunner) (string, error){
		extractDocTextWithAntiword,
		extractDocTextWithTextutil,
	}
	for _, extractor := range extractors {
		text, err := extractor(ctx, absPath, runner)
		if err != nil {
			continue
		}
		if text = normalizeCommandText(text); text != "" {
			return text
		}
	}
	return ""
}

func extractDocTextWithAntiword(ctx context.Context, absPath string, runner wordCommandRunner) (string, error) {
	bin, err := runner.LookPath("antiword")
	if err != nil {
		return "", err
	}
	stdout, stderr, runErr := runner.Run(ctx, bin, absPath)
	if runErr != nil {
		return "", commandError(runErr, stderr)
	}
	return stdout, nil
}

func extractDocTextWithTextutil(ctx context.Context, absPath string, runner wordCommandRunner) (string, error) {
	bin, err := runner.LookPath("textutil")
	if err != nil {
		return "", err
	}
	stdout, stderr, runErr := runner.Run(ctx, bin, "-convert", "txt", "-stdout", absPath)
	if runErr != nil {
		return "", commandError(runErr, stderr)
	}
	return stdout, nil
}

func normalizeCommandText(raw string) string {
	return normalizeDocumentText(raw)
}

func normalizeDocumentText(raw string) string {
	value := strings.ReplaceAll(raw, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	value = strings.ReplaceAll(value, "\x00", "")
	lines := strings.Split(value, "\n")
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		if item := strings.TrimSpace(line); item != "" {
			cleaned = append(cleaned, item)
		}
	}
	return strings.Join(cleaned, "\n")
}

func extractDocumentText(document domain.Document) string {
	if document == nil {
		return ""
	}
	var parts []string
	appendParagraphText(&parts, document.Paragraphs())
	appendTableText(&parts, document.Tables())
	return strings.Join(parts, "\n")
}

func appendParagraphText(parts *[]string, paragraphs []domain.Paragraph) {
	for _, paragraph := range paragraphs {
		if paragraph == nil {
			continue
		}
		if text := strings.TrimSpace(paragraph.Text()); text != "" {
			*parts = append(*parts, text)
		}
	}
}

func appendTableText(parts *[]string, tables []domain.Table) {
	for _, table := range tables {
		if table == nil {
			continue
		}
		for _, row := range table.Rows() {
			if row == nil {
				continue
			}
			for _, cell := range row.Cells() {
				if cell == nil || cell.IsHorizontallyMergedContinuation() {
					continue
				}
				appendParagraphText(parts, cell.Paragraphs())
				appendTableText(parts, cell.Tables())
			}
		}
	}
}

func commandError(err error, stderr string) error {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return errors.New(msg)
	}
	return err
}

func (execWordRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

func (execWordRunner) Run(ctx context.Context, name string, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
