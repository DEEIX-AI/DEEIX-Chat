package conversation

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

const (
	toolTracePresentationMaxInputBytes = 256 * 1024
	toolTracePresentationMaxDepth      = 24
	toolTracePresentationMaxNodes      = 512
	toolTracePresentationMaxSources    = 8
	toolTracePresentationURLChars      = 2048
	toolTracePresentationTitleChars    = 180
	toolTracePresentationSnippet       = 320
	toolTracePresentationTextChars     = 1200
)

type toolTraceSource struct {
	Title   string `json:"title,omitempty"`
	URL     string `json:"url"`
	Snippet string `json:"snippet,omitempty"`
}

type toolTraceOutputPresentation struct {
	Text    string            `json:"text,omitempty"`
	Sources []toolTraceSource `json:"sources,omitempty"`
}

type toolTracePayloadNode struct {
	value interface{}
	depth int
}

var (
	toolTraceMarkdownSourcePattern = regexp.MustCompile(`\[([^\]\r\n]+)\]\((https?://[^\s)]+)\)`)
	toolTraceLabeledSourcePattern  = regexp.MustCompile(`(?im)(?:^|\n)\s*(?:title|标题)\s*[:：]\s*([^\r\n]+)\r?\n\s*(?:url|链接)\s*[:：]\s*(https?://[^\s]+)`)
	toolTraceHTMLImagePattern      = regexp.MustCompile(`(?is)<img\b[^>]*(?:>|$)`)
	toolTraceMarkdownImagePattern  = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	toolTraceBlankLinesPattern     = regexp.MustCompile(`\n{3,}`)
)

func buildToolTraceOutputPresentation(raw string) *toolTraceOutputPresentation {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if len(raw) > toolTracePresentationMaxInputBytes {
		if looksLikeOpaqueToolOutput(raw) || strings.ContainsRune(`{["`, rune(raw[0])) {
			return nil
		}
		raw = tracePresentationSnippet(raw, toolTracePresentationTextChars)
	}
	value := sanitizeOpaqueToolOutput(raw)
	if value == "" {
		return nil
	}

	presentation := &toolTraceOutputPresentation{}
	sourceIndexes := make(map[string]int)
	var payload interface{}
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
				collectToolTraceTextSources(presentation, sourceIndexes, typed)
				if current.depth < toolTracePresentationMaxDepth {
					var embedded interface{}
					if json.Unmarshal([]byte(strings.TrimSpace(typed)), &embedded) == nil {
						pending = append(pending, toolTracePayloadNode{value: embedded, depth: current.depth + 1})
					}
				}
			case []interface{}:
				if current.depth >= toolTracePresentationMaxDepth {
					continue
				}
				for index := len(typed) - 1; index >= 0 && traversed+len(pending) < toolTracePresentationMaxNodes; index-- {
					pending = append(pending, toolTracePayloadNode{value: typed[index], depth: current.depth + 1})
				}
			case map[string]interface{}:
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
				for _, item := range typed {
					if traversed+len(pending) >= toolTracePresentationMaxNodes {
						break
					}
					pending = append(pending, toolTracePayloadNode{value: item, depth: current.depth + 1})
				}
			}
		}
		if presentation.Text == "" {
			presentation.Text = readableJSONPreview(payload)
		}
	}

	presentation.Text = toolTraceHTMLImagePattern.ReplaceAllString(presentation.Text, "")
	presentation.Text = toolTraceMarkdownImagePattern.ReplaceAllString(presentation.Text, "")
	presentation.Text = toolTraceBlankLinesPattern.ReplaceAllString(presentation.Text, "\n\n")
	presentation.Text = tracePresentationSnippet(presentation.Text, toolTracePresentationTextChars)
	if len(presentation.Sources) == 0 && presentation.Text == "" {
		return nil
	}
	return presentation
}

func collectToolTraceRecordSource(
	presentation *toolTraceOutputPresentation,
	indexes map[string]int,
	record map[string]interface{},
) {
	sourceURL := firstToolTracePresentationString(record, "url", "uri", "link", "href", "retrieved_url", "source_url")
	if sourceURL == "" {
		return
	}
	addToolTraceSource(presentation, indexes, toolTraceSource{
		Title:   firstToolTracePresentationString(record, "title", "name", "page_title"),
		URL:     sourceURL,
		Snippet: firstToolTracePresentationString(record, "snippet", "description", "summary", "excerpt"),
	})
}

func collectToolTraceTextSources(
	presentation *toolTraceOutputPresentation,
	indexes map[string]int,
	text string,
) {
	for _, match := range toolTraceLabeledSourcePattern.FindAllStringSubmatch(text, toolTracePresentationMaxSources) {
		if len(match) == 3 {
			addToolTraceSource(presentation, indexes, toolTraceSource{Title: match[1], URL: match[2]})
		}
	}
	for _, match := range toolTraceMarkdownSourcePattern.FindAllStringSubmatch(text, toolTracePresentationMaxSources) {
		if len(match) == 3 {
			addToolTraceSource(presentation, indexes, toolTraceSource{Title: match[1], URL: match[2]})
		}
	}
}

func addToolTraceSource(
	presentation *toolTraceOutputPresentation,
	indexes map[string]int,
	source toolTraceSource,
) {
	source.URL = normalizeToolTraceSourceURL(source.URL)
	if source.URL == "" {
		return
	}
	source.Title = tracePresentationSnippet(source.Title, toolTracePresentationTitleChars)
	source.Snippet = tracePresentationSnippet(source.Snippet, toolTracePresentationSnippet)
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

func firstToolTracePresentationString(record map[string]interface{}, keys ...string) string {
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

func tracePresentationSnippet(value string, maxChars int) string {
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
