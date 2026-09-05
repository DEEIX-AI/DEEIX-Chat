package compact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	domainconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/tokenestimate"
)

// SnapshotBoundaryIndex returns the covered boundary index when a snapshot can
// be proven to match the current active branch prefix.
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

// SnapshotBoundaryAncestorIndex returns the snapshot boundary index inside a
// contiguous ancestor path. It is used when the loaded path starts at or after
// the original branch root, so the full covered-prefix hash cannot be
// recomputed locally. Parent links are immutable, so a matching boundary
// message in the current ancestor path proves the snapshot belongs to this
// branch.
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

// SnapshotHasCoverage rejects legacy snapshots that do not carry a verifiable
// branch boundary. Such snapshots may still be shown in traces, but must not
// replace history in the model prompt.
func SnapshotHasCoverage(snapshot *domainconversation.ContextSnapshot) bool {
	return snapshot != nil &&
		strings.TrimSpace(snapshot.SummaryText) != "" &&
		snapshot.CoveredUntilMessageID > 0 &&
		strings.TrimSpace(snapshot.CoveredUntilPublicID) != "" &&
		strings.TrimSpace(snapshot.CoveragePathHash) != "" &&
		snapshot.CoveredMessageCount > 0
}

// CoveragePathHash hashes the exact covered prefix of a branch. The hash uses
// stable message identity and parent links, not message content, so edits create
// a new message path rather than mutating previous coverage.
func CoveragePathHash(messages []domainconversation.Message) string {
	return ExtendCoveragePathHash("", messages)
}

// ExtendCoveragePathHash appends a new covered segment to an existing coverage
// hash. This keeps rolling snapshots verifiable even when only the previous
// boundary-to-current ancestor window is loaded.
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
