package toolresult

import (
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

const (
	MaxPresentationInputBytes       = 256 * 1024
	toolTracePresentationMaxDepth   = 24
	toolTracePresentationMaxNodes   = 512
	toolTracePresentationMaxSources = 8
	toolTracePresentationURLChars   = 2048
	toolTracePresentationTitleChars = 180
	toolTracePresentationSnippet    = 320
	MaxPresentationTextTokens       = 512
)

type Source struct {
	Title   string `json:"title,omitempty"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type Presentation struct {
	Text    string   `json:"text,omitempty"`
	Sources []Source `json:"sources,omitempty"`
}

type toolTracePayloadNode struct {
	value any
	depth int
}

var (
	toolTraceMarkdownSourcePattern = regexp.MustCompile(`\[([^\]\r\n]+)\]\((https?://[^\s)]+)\)`)
	toolTraceLabeledSourcePattern  = regexp.MustCompile(`(?im)(?:^|\n)\s*(?:title|标题)\s*[:：]\s*([^\r\n]+)\r?\n\s*(?:url|链接)\s*[:：]\s*(https?://[^\s]+)`)
	toolTraceHTMLImagePattern      = regexp.MustCompile(`(?is)<img\b[^>]*(?:>|$)`)
	toolTraceMarkdownImagePattern  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	toolTraceBlankLinesPattern     = regexp.MustCompile(`\n{3,}`)
)

func BuildPresentation(raw string) *Presentation {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > MaxPresentationInputBytes {
		if looksLikeOpaque(raw) || strings.ContainsRune(`{["`, rune(raw[0])) {
			return nil
		}
		raw = tokenSnippet(raw, MaxPresentationTextTokens)
	}
	value := SanitizeOpaque(raw)
	if value == "" {
		return nil
	}

	presentation := &Presentation{}
	sourceIndexes := make(map[string]int)
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		presentation.Text = value
		collectToolTraceTextSources(presentation, sourceIndexes, value)
	} else {
		pending := []toolTracePayloadNode{{value: payload}}
		traversed := 0

		for len(pending) > 0 && traversed < toolTracePresentationMaxNodes {
			current := pending[len(pending)-1]
			pending = pending[:len(pending)-1]
			traversed++

			switch typed := current.value.(type) {
			case string:
				narrative, embeddedPayloads := toolTraceNarrativeText(typed)
				collectToolTraceTextSources(presentation, sourceIndexes, narrative)
				if current.depth < toolTracePresentationMaxDepth {
					pending = appendToolTracePayloadNodes(pending, embeddedPayloads, current.depth+1)
				}
			case []any:
				if current.depth >= toolTracePresentationMaxDepth {
					continue
				}
				pending = appendToolTracePayloadNodes(pending, typed, current.depth+1)
			case map[string]any:
				collectToolTraceRecordSource(presentation, sourceIndexes, typed)
				if presentation.Text == "" && strings.EqualFold(firstToolTracePresentationString(typed, "type"), "text") {
					presentation.Text = firstToolTracePresentationString(typed, "text")
				}
				if current.depth == 0 && presentation.Text == "" {
					presentation.Text = firstToolTracePresentationString(typed, "answer", "summary", "message")
				}
				if current.depth >= toolTracePresentationMaxDepth {
					continue
				}
				items := orderedToolTraceRecordValues(typed)
				pending = appendToolTracePayloadNodes(pending, items, current.depth+1)
			}
		}
		if presentation.Text == "" {
			presentation.Text = ReadablePreview(payload)
		}
	}

	presentation.Text, _ = toolTraceNarrativeText(presentation.Text)
	presentation.Text = toolTraceHTMLImagePattern.ReplaceAllString(presentation.Text, "")
	presentation.Text = toolTraceMarkdownImagePattern.ReplaceAllString(presentation.Text, "")
	presentation.Text = toolTraceBlankLinesPattern.ReplaceAllString(presentation.Text, "\n\n")
	presentation.Text = tokenSnippet(presentation.Text, MaxPresentationTextTokens)
	if len(presentation.Sources) == 0 && presentation.Text == "" {
		return nil
	}
	return presentation
}

func appendToolTracePayloadNodes(
	pending []toolTracePayloadNode,
	values []any,
	depth int,
) []toolTracePayloadNode {
	available := toolTracePresentationMaxNodes - len(pending)
	if available <= 0 {
		return pending
	}
	if len(values) > available {
		values = values[:available]
	}
	for index := len(values) - 1; index >= 0; index-- {
		pending = append(pending, toolTracePayloadNode{value: values[index], depth: depth})
	}
	return pending
}

func orderedToolTraceRecordValues(record map[string]any) []any {
	keys := make([]string, 0, len(record))
	for key := range record {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(left, right int) bool {
		leftRank := toolTracePresentationKeyRank(keys[left])
		rightRank := toolTracePresentationKeyRank(keys[right])
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		return keys[left] < keys[right]
	})

	values := make([]any, 0, len(keys))
	for _, key := range keys {
		values = append(values, record[key])
	}
	return values
}

func toolTracePresentationKeyRank(key string) int {
	normalized := normalizeToolTracePresentationKey(key)
	switch normalized {
	case "content":
		return 0
	case "answer":
		return 1
	case "summary":
		return 2
	case "message":
		return 3
	case "result":
		return 4
	case "structured_content":
		return 5
	case "results":
		return 6
	case "items":
		return 7
	case "data":
		return 8
	case "sources":
		return 9
	case "citations":
		return 10
	default:
		return 11
	}
}

// toolTraceNarrativeText separates standalone structured payloads from the
// human-readable Markdown around them. Raw payloads remain available through
// the tool result detail; the compact presentation only carries narrative text.
func toolTraceNarrativeText(value string) (string, []any) {
	var narrative strings.Builder
	payloads := make([]any, 0, 1)
	fenceMarker := byte(0)
	fenceWidth := 0

	for offset := 0; offset < len(value); {
		lineEnd := strings.IndexByte(value[offset:], '\n')
		if lineEnd < 0 {
			lineEnd = len(value)
		} else {
			lineEnd += offset
		}

		line := value[offset:lineEnd]
		trimmed := strings.TrimLeft(line, " \t")
		indentWidth := len(line) - len(trimmed)
		if marker, width := toolTraceMarkdownFence(trimmed); marker != 0 {
			if fenceMarker == 0 {
				fenceMarker = marker
				fenceWidth = width
			} else if marker == fenceMarker && width >= fenceWidth {
				fenceMarker = 0
				fenceWidth = 0
			}
		}

		if fenceMarker == 0 && indentWidth < 4 && len(trimmed) > 0 &&
			(trimmed[0] == '{' || trimmed[0] == '[') {
			var payload any
			decoder := json.NewDecoder(strings.NewReader(value[offset+indentWidth:]))
			if err := decoder.Decode(&payload); err == nil {
				switch payload.(type) {
				case map[string]any, []any:
					payloads = append(payloads, payload)
					offset += indentWidth + int(decoder.InputOffset())
					continue
				}
			}
		}

		narrative.WriteString(line)
		if lineEnd < len(value) {
			narrative.WriteByte('\n')
			offset = lineEnd + 1
		} else {
			offset = lineEnd
		}
	}

	text := strings.TrimSpace(narrative.String())
	if text == "" {
		for _, payload := range payloads {
			if text = strings.TrimSpace(ReadablePreview(payload)); text != "" {
				break
			}
		}
	}
	return text, payloads
}

func toolTraceMarkdownFence(value string) (byte, int) {
	if len(value) < 3 || (value[0] != '`' && value[0] != '~') {
		return 0, 0
	}
	marker := value[0]
	width := 1
	for width < len(value) && value[width] == marker {
		width++
	}
	if width < 3 {
		return 0, 0
	}
	return marker, width
}

func collectToolTraceRecordSource(
	presentation *Presentation,
	indexes map[string]int,
	record map[string]any,
) {
	sourceURL := firstToolTracePresentationString(record, "url", "uri", "link", "href", "retrieved_url", "source_url")
	if sourceURL == "" {
		return
	}
	addToolTraceSource(presentation, indexes, Source{
		Title:   firstToolTracePresentationString(record, "title", "name", "page_title"),
		URL:     sourceURL,
		Snippet: firstToolTracePresentationString(record, "snippet", "description", "summary", "excerpt"),
	})
}

func collectToolTraceTextSources(
	presentation *Presentation,
	indexes map[string]int,
	text string,
) {
	for _, match := range toolTraceLabeledSourcePattern.FindAllStringSubmatch(text, toolTracePresentationMaxSources) {
		if len(match) == 3 {
			addToolTraceSource(presentation, indexes, Source{Title: match[1], URL: match[2]})
		}
	}
	for _, match := range toolTraceMarkdownSourcePattern.FindAllStringSubmatch(text, toolTracePresentationMaxSources) {
		if len(match) == 3 {
			addToolTraceSource(presentation, indexes, Source{Title: match[1], URL: match[2]})
		}
	}
}

func addToolTraceSource(
	presentation *Presentation,
	indexes map[string]int,
	source Source,
) {
	source.URL = normalizeToolTraceSourceURL(source.URL)
	if source.URL == "" {
		return
	}
	source.Title = Snippet(source.Title, toolTracePresentationTitleChars)
	source.Snippet = Snippet(source.Snippet, toolTracePresentationSnippet)
	if index, ok := indexes[source.URL]; ok {
		current := &presentation.Sources[index]
		if current.Title == "" {
			current.Title = source.Title
		}
		if current.Snippet == "" {
			current.Snippet = source.Snippet
		}
		return
	}
	if len(presentation.Sources) >= toolTracePresentationMaxSources {
		return
	}
	indexes[source.URL] = len(presentation.Sources)
	presentation.Sources = append(presentation.Sources, source)
}

func normalizeToolTraceSourceURL(raw string) string {
	value := strings.Trim(strings.TrimSpace(raw), `.,;:!?，。；：！？)]}>'"`)
	if len([]rune(value)) > toolTracePresentationURLChars {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ""
	}
	return value
}

func firstToolTracePresentationString(record map[string]any, keys ...string) string {
	for _, key := range keys {
		for candidate, raw := range record {
			if normalizeToolTracePresentationKey(candidate) != key {
				continue
			}
			if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		}
	}
	return ""
}

func normalizeToolTracePresentationKey(value string) string {
	var normalized strings.Builder
	previousSeparator := true
	previousLowerOrDigit := false
	for _, char := range strings.TrimSpace(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			if unicode.IsUpper(char) && previousLowerOrDigit && !previousSeparator {
				normalized.WriteByte('_')
			}
			normalized.WriteRune(unicode.ToLower(char))
			previousSeparator = false
			previousLowerOrDigit = unicode.IsLower(char) || unicode.IsDigit(char)
			continue
		}
		if !previousSeparator && normalized.Len() > 0 {
			normalized.WriteByte('_')
		}
		previousSeparator = true
		previousLowerOrDigit = false
	}
	return strings.Trim(normalized.String(), "_")
}

func Snippet(value string, maxChars int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value
	}
	return strings.TrimSpace(string(runes[:maxChars])) + "…"
}

func tokenSnippet(value string, maxTokens int64) string {
	value = strings.TrimSpace(value)
	if value == "" || maxTokens <= 0 || tokenestimate.Estimate(value) <= maxTokens {
		return value
	}
	runes := []rune(value)
	low := 0
	high := len(runes)
	for low < high {
		middle := (low + high + 1) / 2
		if tokenestimate.Estimate(strings.TrimSpace(string(runes[:middle]))+"…") <= maxTokens {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return strings.TrimSpace(string(runes[:low])) + "…"
}

// ReadablePreview returns the first useful narrative value from generic JSON.
func ReadablePreview(value any) string {
	switch typed := value.(type) {
	case []any:
		parts := make([]string, 0, min(len(typed), 3))
		for _, item := range typed {
			if text := ReadablePreview(item); text != "" {
				parts = append(parts, text)
			}
			if len(parts) >= 3 {
				break
			}
		}
		return strings.Join(parts, "；")
	case map[string]any:
		for _, key := range []string{"summary", "answer", "text", "content", "message", "result"} {
			if text := stringValue(typed[key]); text != "" {
				return text
			}
		}
		if title := stringValue(typed["title"]); title != "" {
			if sourceURL := stringValue(typed["url"]); sourceURL != "" {
				return title + " " + sourceURL
			}
			return title
		}
		for _, key := range []string{"url", "uri", "link"} {
			if text := stringValue(typed[key]); text != "" {
				return text
			}
		}
		for _, key := range []string{"results", "items", "data", "sources", "citations"} {
			if text := ReadablePreview(typed[key]); text != "" {
				return text
			}
		}
	case string:
		return typed
	}
	return ""
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}
