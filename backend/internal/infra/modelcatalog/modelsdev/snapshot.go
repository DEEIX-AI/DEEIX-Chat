package modelsdev

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	domainchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/channel"
)

// reasoningSnapshotVersion 是精简推理目录快照的格式版本，格式不兼容时递增。
const reasoningSnapshotVersion = 1

// maxReasoningSnapshotBytes 限制快照解压后的大小，防止异常输入耗尽内存。
const maxReasoningSnapshotBytes = 32 << 20

var (
	// ErrReasoningCatalogEmpty 表示目录中没有任何可用的推理模型条目。
	ErrReasoningCatalogEmpty = errors.New("reasoning catalog is empty")
	// ErrReasoningCatalogMalformed 表示目录或快照不是预期的 JSON 结构。
	ErrReasoningCatalogMalformed = errors.New("reasoning catalog is malformed")
)

type modelsDevReasoningModel struct {
	ID               string                     `json:"id"`
	CanonicalModelID string                     `json:"canonical_model_id"`
	Reasoning        bool                       `json:"reasoning"`
	ReasoningOptions []modelsDevReasoningOption `json:"reasoning_options"`
}

type modelsDevReasoningOption struct {
	Type   string   `json:"type"`
	Values []any    `json:"values"`
	Min    *float64 `json:"min"`
	Max    *float64 `json:"max"`
}

// ParseReasoningCatalog 把 models.dev api.json 精简为推理目录条目：只保留 reasoning 为 true 且带有
// 可识别 reasoning_options 的模型。单个 provider 或模型结构异常时跳过，不影响其余条目；结果按 provider 与
// 模型 id 排序，保证快照内容稳定。
func ParseReasoningCatalog(raw []byte) ([]domainchannel.ReasoningCatalogEntry, error) {
	var providers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &providers); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReasoningCatalogMalformed, err)
	}
	entries := make([]domainchannel.ReasoningCatalogEntry, 0, 4096)
	for providerID, providerRaw := range providers {
		provider := strings.TrimSpace(providerID)
		var payload struct {
			Models map[string]json.RawMessage `json:"models"`
		}
		if provider == "" || json.Unmarshal(providerRaw, &payload) != nil {
			continue
		}
		for modelKey, modelRaw := range payload.Models {
			var model modelsDevReasoningModel
			if json.Unmarshal(modelRaw, &model) != nil || !model.Reasoning {
				continue
			}
			modelID := strings.TrimSpace(model.ID)
			if modelID == "" {
				modelID = strings.TrimSpace(modelKey)
			}
			options := normalizeModelsDevReasoningOptions(model.ReasoningOptions)
			if modelID == "" || len(options) == 0 {
				continue
			}
			entries = append(entries, domainchannel.ReasoningCatalogEntry{
				Provider:    provider,
				ModelID:     modelID,
				CanonicalID: strings.TrimSpace(model.CanonicalModelID),
				Reasoning:   true,
				Options:     options,
			})
		}
	}
	if len(entries) == 0 {
		return nil, ErrReasoningCatalogEmpty
	}
	sortReasoningCatalogEntries(entries)
	return entries, nil
}

// normalizeModelsDevReasoningOptions 保留可识别的选项类型：effort 至少要有一个字符串取值，
// budget_tokens 的区间只接受非负整数，同类型选项只保留首个。
func normalizeModelsDevReasoningOptions(raw []modelsDevReasoningOption) []domainchannel.ReasoningCatalogOption {
	options := make([]domainchannel.ReasoningCatalogOption, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, item := range raw {
		optionType := strings.ToLower(strings.TrimSpace(item.Type))
		if _, duplicated := seen[optionType]; duplicated {
			continue
		}
		option := domainchannel.ReasoningCatalogOption{Type: optionType}
		switch optionType {
		case domainchannel.ReasoningCatalogOptionEffort:
			for _, value := range item.Values {
				if text, ok := value.(string); ok && strings.TrimSpace(text) != "" {
					option.Values = append(option.Values, strings.ToLower(strings.TrimSpace(text)))
				}
			}
			if len(option.Values) == 0 {
				continue
			}
		case domainchannel.ReasoningCatalogOptionBudget:
			option.Min = modelsDevBudgetBound(item.Min)
			option.Max = modelsDevBudgetBound(item.Max)
		case domainchannel.ReasoningCatalogOptionToggle:
		default:
			continue
		}
		seen[optionType] = struct{}{}
		options = append(options, option)
	}
	return options
}

func modelsDevBudgetBound(value *float64) *int {
	if value == nil || *value < 0 || *value > math.MaxInt32 || *value != math.Trunc(*value) {
		return nil
	}
	bound := int(*value)
	return &bound
}

func sortReasoningCatalogEntries(entries []domainchannel.ReasoningCatalogEntry) {
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Provider != entries[right].Provider {
			return entries[left].Provider < entries[right].Provider
		}
		return entries[left].ModelID < entries[right].ModelID
	})
}

// reasoningCatalogSnapshotFile 是快照的序列化结构，storage 缓存（明文）与内置快照（gzip）共用。
type reasoningCatalogSnapshotFile struct {
	Version   int                             `json:"version"`
	Source    string                          `json:"source"`
	Origin    string                          `json:"origin,omitempty"`
	FetchedAt time.Time                       `json:"fetchedAt"`
	Models    []reasoningCatalogSnapshotModel `json:"models"`
}

type reasoningCatalogSnapshotModel struct {
	Provider    string                           `json:"provider"`
	ID          string                           `json:"id"`
	CanonicalID string                           `json:"canonicalId,omitempty"`
	Options     []reasoningCatalogSnapshotOption `json:"options"`
}

type reasoningCatalogSnapshotOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    *int     `json:"min,omitempty"`
	Max    *int     `json:"max,omitempty"`
}

// MarshalReasoningSnapshot 把快照序列化为缩进的明文 JSON，用于 storage 缓存，便于运维查看。
func MarshalReasoningSnapshot(snapshot domainchannel.ReasoningCatalogSnapshot) ([]byte, error) {
	file, err := newReasoningSnapshotFile(snapshot)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(file, "", "  ")
}

// EncodeReasoningSnapshot 把快照序列化为 gzip 压缩的 JSON，用于内置快照；gzip 头不带时间戳，相同输入产出相同字节。
func EncodeReasoningSnapshot(snapshot domainchannel.ReasoningCatalogSnapshot) ([]byte, error) {
	file, err := newReasoningSnapshotFile(snapshot)
	if err != nil {
		return nil, err
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(writer).Encode(file); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func newReasoningSnapshotFile(snapshot domainchannel.ReasoningCatalogSnapshot) (reasoningCatalogSnapshotFile, error) {
	if len(snapshot.Entries) == 0 {
		return reasoningCatalogSnapshotFile{}, ErrReasoningCatalogEmpty
	}
	file := reasoningCatalogSnapshotFile{
		Version:   reasoningSnapshotVersion,
		Source:    strings.TrimSpace(snapshot.Source),
		Origin:    strings.TrimSpace(snapshot.Origin),
		FetchedAt: snapshot.FetchedAt.UTC(),
		Models:    make([]reasoningCatalogSnapshotModel, 0, len(snapshot.Entries)),
	}
	for _, entry := range snapshot.Entries {
		model := reasoningCatalogSnapshotModel{
			Provider:    entry.Provider,
			ID:          entry.ModelID,
			CanonicalID: entry.CanonicalID,
			Options:     make([]reasoningCatalogSnapshotOption, 0, len(entry.Options)),
		}
		for _, option := range entry.Options {
			model.Options = append(model.Options, reasoningCatalogSnapshotOption{
				Type:   option.Type,
				Values: option.Values,
				Min:    option.Min,
				Max:    option.Max,
			})
		}
		file.Models = append(file.Models, model)
	}
	return file, nil
}

// DecodeReasoningSnapshot 解析 EncodeReasoningSnapshot（gzip）或 MarshalReasoningSnapshot（明文）的输出。
// 版本不符、结构异常或没有条目时返回错误，调用方应回退到其他来源。
func DecodeReasoningSnapshot(data []byte) (domainchannel.ReasoningCatalogSnapshot, error) {
	var reader io.Reader = bytes.NewReader(data)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzipReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return domainchannel.ReasoningCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrReasoningCatalogMalformed, err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	plain, err := io.ReadAll(io.LimitReader(reader, maxReasoningSnapshotBytes+1))
	if err != nil {
		return domainchannel.ReasoningCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrReasoningCatalogMalformed, err)
	}
	if len(plain) > maxReasoningSnapshotBytes {
		return domainchannel.ReasoningCatalogSnapshot{}, fmt.Errorf("%w: snapshot exceeds %d bytes", ErrReasoningCatalogMalformed, maxReasoningSnapshotBytes)
	}
	var file reasoningCatalogSnapshotFile
	if err := json.Unmarshal(plain, &file); err != nil {
		return domainchannel.ReasoningCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrReasoningCatalogMalformed, err)
	}
	if file.Version != reasoningSnapshotVersion {
		return domainchannel.ReasoningCatalogSnapshot{}, fmt.Errorf("%w: unsupported snapshot version %d", ErrReasoningCatalogMalformed, file.Version)
	}
	snapshot := domainchannel.ReasoningCatalogSnapshot{
		Source:    file.Source,
		Origin:    file.Origin,
		FetchedAt: file.FetchedAt,
		Entries:   make([]domainchannel.ReasoningCatalogEntry, 0, len(file.Models)),
	}
	for _, model := range file.Models {
		if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" || len(model.Options) == 0 {
			continue
		}
		entry := domainchannel.ReasoningCatalogEntry{
			Provider:    model.Provider,
			ModelID:     model.ID,
			CanonicalID: model.CanonicalID,
			Reasoning:   true,
			Options:     make([]domainchannel.ReasoningCatalogOption, 0, len(model.Options)),
		}
		for _, option := range model.Options {
			entry.Options = append(entry.Options, domainchannel.ReasoningCatalogOption{
				Type:   option.Type,
				Values: option.Values,
				Min:    option.Min,
				Max:    option.Max,
			})
		}
		snapshot.Entries = append(snapshot.Entries, entry)
	}
	if len(snapshot.Entries) == 0 {
		return domainchannel.ReasoningCatalogSnapshot{}, ErrReasoningCatalogEmpty
	}
	return snapshot, nil
}
