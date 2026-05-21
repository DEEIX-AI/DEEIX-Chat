package builtin

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coregx/gxpdf"
	"golang.org/x/text/unicode/norm"
)

// PDFTextPage 表示单页 PDF 原生文本提取结果。
type PDFTextPage struct {
	PageNumber    int
	Text          string
	ExtractFailed bool
}

// PDFTextResult 表示 PDF 原生提取结果。
type PDFTextResult struct {
	PageCount int
	Pages     []PDFTextPage
}

// ExtractPDFText 从 PDF 文件中提取纯文本。
func ExtractPDFText(absPath string, maxPages int) (string, error) {
	result, err := ExtractPDFPages(absPath, maxPages)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(result.Pages))
	for _, page := range result.Pages {
		value := strings.TrimSpace(page.Text)
		if value == "" {
			continue
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n"), nil
}

// ExtractPDFPages 从 PDF 文件中逐页提取纯文本。
func ExtractPDFPages(absPath string, maxPages int) (PDFTextResult, error) {
	return extractPDFPagesWithGXPDF(absPath, maxPages)
}

func extractPDFPagesWithGXPDF(absPath string, maxPages int) (PDFTextResult, error) {
	doc, err := gxpdf.Open(strings.TrimSpace(absPath))
	if err != nil {
		return PDFTextResult{}, fmt.Errorf("pdf_parse_failed: %w", err)
	}
	defer doc.Close() //nolint:errcheck

	totalPages := doc.PageCount()
	if totalPages <= 0 {
		return PDFTextResult{}, nil
	}
	limit := totalPages
	if maxPages > 0 && limit > maxPages {
		limit = maxPages
	}

	pages := make([]PDFTextPage, 0, limit)
	for pageNum := 1; pageNum <= limit; pageNum++ {
		text, extractErr := doc.ExtractTextFromPage(pageNum)
		if extractErr != nil {
			pages = append(pages, PDFTextPage{
				PageNumber:    pageNum,
				Text:          "",
				ExtractFailed: true,
			})
			continue
		}
		pages = append(pages, PDFTextPage{
			PageNumber:    pageNum,
			Text:          normalizeExtractedPDFText(text),
			ExtractFailed: false,
		})
	}

	return PDFTextResult{
		PageCount: totalPages,
		Pages:     pages,
	}, nil
}

// DetectPDFPageCount 探测 PDF 页数。
func DetectPDFPageCount(absPath string) int {
	doc, err := gxpdf.Open(strings.TrimSpace(absPath))
	if err != nil {
		return 0
	}
	defer doc.Close() //nolint:errcheck
	return doc.PageCount()
}

func normalizeExtractedPDFText(raw string) string {
	value := strings.ReplaceAll(raw, "\x00", "")
	value = norm.NFKC.String(value)
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			continue
		}
		line = mergeSpacedSingleRuneTokens(line)
		line = tightenExtractedSpacing(line)
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		normalized = append(normalized, line)
	}

	return strings.Join(normalized, "\n")
}

func mergeSpacedSingleRuneTokens(line string) string {
	fields := strings.Fields(line)
	if len(fields) <= 1 {
		return line
	}

	merged := make([]string, 0, len(fields))
	for i := 0; i < len(fields); {
		if !isSingleRuneJoinable(fields[i]) {
			merged = append(merged, fields[i])
			i++
			continue
		}

		j := i + 1
		for j < len(fields) && isSingleRuneJoinable(fields[j]) {
			j++
		}

		run := fields[i:j]
		if shouldMergeSingleRuneRun(run) {
			merged = append(merged, strings.Join(run, ""))
		} else {
			merged = append(merged, run...)
		}
		i = j
	}

	return strings.Join(merged, " ")
}

func isSingleRuneJoinable(token string) bool {
	r, size := utf8.DecodeRuneInString(token)
	if r == utf8.RuneError || size != len(token) {
		return false
	}
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	return strings.ContainsRune(`._:/\-+%#@&()[]{}'"`, r)
}

func shouldMergeSingleRuneRun(run []string) bool {
	if len(run) >= 3 {
		return true
	}
	if len(run) != 2 {
		return false
	}

	a, _ := utf8.DecodeRuneInString(run[0])
	b, _ := utf8.DecodeRuneInString(run[1])
	if isWordConnectorRune(a) || isWordConnectorRune(b) {
		return true
	}
	if unicode.IsDigit(a) && unicode.IsDigit(b) {
		return true
	}
	if unicode.IsUpper(a) && unicode.IsUpper(b) {
		return true
	}
	return false
}

func tightenExtractedSpacing(line string) string {
	runes := []rune(line)
	var b strings.Builder
	b.Grow(len(line))

	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if !unicode.IsSpace(r) {
			b.WriteRune(r)
			continue
		}

		prev, hasPrev := previousNonSpaceRune(runes, i)
		next, hasNext := nextNonSpaceRune(runes, i)
		if !hasPrev || !hasNext {
			continue
		}
		if shouldTightenSpacing(prev, next) {
			continue
		}

		if b.Len() > 0 {
			b.WriteByte(' ')
		}
	}

	return b.String()
}

func previousNonSpaceRune(runes []rune, idx int) (rune, bool) {
	for i := idx - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

func nextNonSpaceRune(runes []rune, idx int) (rune, bool) {
	for i := idx + 1; i < len(runes); i++ {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

func shouldTightenSpacing(prev, next rune) bool {
	if isCJKLikeRune(prev) && (isCJKLikeRune(next) || unicode.IsDigit(next) || isTightPunctuation(next)) {
		return true
	}
	if isCJKLikeRune(next) && (unicode.IsDigit(prev) || isTightPunctuation(prev)) {
		return true
	}
	if unicode.IsDigit(prev) && unicode.IsDigit(next) {
		return false
	}
	if isWordConnectorRune(prev) || isWordConnectorRune(next) {
		return true
	}
	if isOpeningPunctuation(prev) || isClosingPunctuation(next) {
		return true
	}
	return false
}

func isCJKLikeRune(r rune) bool {
	return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

func isWordConnectorRune(r rune) bool {
	return strings.ContainsRune("._:/\\-+%#@&", r)
}

func isOpeningPunctuation(r rune) bool {
	return strings.ContainsRune("([{\"'“‘《〈「『【", r)
}

func isClosingPunctuation(r rune) bool {
	return strings.ContainsRune(".,:;!?%)]}\"'”’》〉」』】、，。：；！？", r)
}

func isTightPunctuation(r rune) bool {
	return isOpeningPunctuation(r) || isClosingPunctuation(r)
}
