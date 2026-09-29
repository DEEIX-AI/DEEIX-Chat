package compact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

// SnapshotBoundaryIndex 在能证明快照与当前活动分支前缀匹配时，
// 返回其覆盖的边界索引。
func SnapshotBoundaryIndex(messages []domainconversation.Message, snapshot *domainconversation.ContextSnapshot) (int, bool) {
	if !SnapshotHasCoverage(snapshot) || len(messages) == 0 {
		return -1, false
	}
	for index, message := range messages {
		if message.ID != snapshot.CoveredUntilMessageID {
			continue
		}
		if strings.TrimSpace(message.PublicID) != strings.TrimSpace(snapshot.CoveredUntilPublicID) {
			return -1, false
		}
		coveredCount := index + 1
		if coveredCount != snapshot.CoveredMessageCount {
			return -1, false
		}
		if CoveragePathHash(messages[:coveredCount]) != strings.TrimSpace(snapshot.CoveragePathHash) {
			return -1, false
		}
		return index, true
	}
	return -1, false
}

// SnapshotBoundaryAncestorIndex 返回快照边界在连续祖先路径中的索引。
// 适用于已加载路径起始于原始分支根或其之后、因而无法在本地重新计算完整覆盖前缀哈希的场景。
// 父链接不可变，因此只要当前祖先路径中存在匹配的边界消息，即可证明该快照属于此分支。
func SnapshotBoundaryAncestorIndex(messages []domainconversation.Message, snapshot *domainconversation.ContextSnapshot) (int, bool) {
	if !SnapshotHasCoverage(snapshot) || len(messages) == 0 {
		return -1, false
	}
	for index, message := range messages {
		if message.ID != snapshot.CoveredUntilMessageID {
			continue
		}
		if strings.TrimSpace(message.PublicID) != strings.TrimSpace(snapshot.CoveredUntilPublicID) {
			return -1, false
		}
		return index, true
	}
	return -1, false
}

// SnapshotHasCoverage 拒绝不带可验证分支边界的
// 旧版快照。此类快照仍可在 trace 中展示，但不得
// 替换模型提示词中的历史。
func SnapshotHasCoverage(snapshot *domainconversation.ContextSnapshot) bool {
	return snapshot != nil &&
		strings.TrimSpace(snapshot.SummaryText) != "" &&
		snapshot.CoveredUntilMessageID > 0 &&
		strings.TrimSpace(snapshot.CoveredUntilPublicID) != "" &&
		strings.TrimSpace(snapshot.CoveragePathHash) != "" &&
		snapshot.CoveredMessageCount > 0
}

// CoveragePathHash 对分支的精确覆盖前缀计算哈希。该哈希使用
// 稳定的消息身份与父链接而非消息内容，因此编辑会创建
// 新的消息路径，而不是改变已有覆盖范围。
func CoveragePathHash(messages []domainconversation.Message) string {
	return ExtendCoveragePathHash("", messages)
}

// ExtendCoveragePathHash 将新的覆盖片段追加到已有的覆盖
// 哈希上。这使滚动快照即使只加载了上一个
// 边界到当前的祖先窗口，也仍可验证。
func ExtendCoveragePathHash(previousHash string, messages []domainconversation.Message) string {
	state := strings.TrimSpace(previousHash)
	for _, message := range messages {
		hash := sha256.New()
		_, _ = hash.Write([]byte(state))
		_, _ = hash.Write([]byte{0})
		parentID := uint(0)
		if message.ParentMessageID != nil {
			parentID = *message.ParentMessageID
		}
		_, _ = fmt.Fprintf(
			hash,
			"%d:%s:%d:%s\n",
			message.ID,
			strings.TrimSpace(message.PublicID),
			parentID,
			strings.TrimSpace(message.Role),
		)
		state = hex.EncodeToString(hash.Sum(nil))
	}
	return state
}

func splitMessagesByPreservedTurns(messages []domainconversation.Message, preserveTurns int) ([]domainconversation.Message, []domainconversation.Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	if preserveTurns <= 0 {
		preserveTurns = 8
	}

	userTurns := 0
	firstPreservedUserIndex := -1
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Role != "user" {
			continue
		}
		userTurns++
		if userTurns <= preserveTurns {
			firstPreservedUserIndex = index
			continue
		}
		break
	}
	if userTurns <= preserveTurns || firstPreservedUserIndex <= 0 {
		return nil, messages
	}

	covered := append([]domainconversation.Message(nil), messages[:firstPreservedUserIndex]...)
	retained := append([]domainconversation.Message(nil), messages[firstPreservedUserIndex:]...)
	return covered, retained
}

func countUserTurns(messages []domainconversation.Message) int {
	count := 0
	for _, message := range messages {
		if message.Role == "user" {
			count++
		}
	}
	return count
}

func estimateMessageTokenTotal(messages []domainconversation.Message) int64 {
	var total int64
	for _, message := range messages {
		if message.TokenUsage > 0 {
			total += message.TokenUsage
			continue
		}
		total += tokenestimate.Estimate(message.Content) + 5
	}
	return total
}
