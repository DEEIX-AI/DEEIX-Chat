package embeddingutil

import "strings"

// ChunkText 将文本按估算 token 数分片，使用段落优先截断策略。
// chunkSize 和 overlap 的单位为 token，按 2 bytes/token 估算。
func ChunkText(text string, chunkSize, overlap int) []string {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if overlap < 0 {
		overlap = 64
	}
	// 估算：CJK 约 1.5 chars/token，ASCII 约 4 chars/token，取折中 2 chars/token。
	// 这里按 rune 切分，不能使用字符串字节下标，否则中文文本会出现 slice 越界。
	chunkRunes := chunkSize * 2
	overlapRunes := overlap * 2
	if overlapRunes >= chunkRunes {
		overlapRunes = chunkRunes / 4
	}
	paragraphBreak := []rune("\n\n")
	lineBreak := []rune("\n")

	runes := []rune(text)
	if len(runes) <= chunkRunes {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + chunkRunes
		if end > len(runes) {
			end = len(runes)
		}
		slice := string(runes[start:end])
		if end < len(runes) {
			window := runes[start:end]
			if idx := lastRuneSequenceIndex(window, paragraphBreak); idx > chunkRunes/2 {
				end = start + idx + 2
				slice = string(runes[start:end])
			} else if idx := lastRuneSequenceIndex(window, lineBreak); idx > chunkRunes/2 {
				end = start + idx + 1
				slice = string(runes[start:end])
			}
		}
		if strings.TrimSpace(slice) != "" {
			chunks = append(chunks, slice)
		}
		if end >= len(runes) {
			break
		}
		next := end - overlapRunes
		if next <= start {
			next = start + 1
		}
		start = next
	}
	return chunks
}

func lastRuneSequenceIndex(haystack []rune, needle []rune) int {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return -1
	}
	for i := len(haystack) - len(needle); i >= 0; i-- {
		matched := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}
