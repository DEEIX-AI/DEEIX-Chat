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

// snapshotVersion 是精简目录快照的格式版本，格式不兼容时递增。
// v2 起每个模型可同时带推理选项与输入模态；v1 快照只有推理数据，读取时视为不兼容，由内置快照重新写入。
const snapshotVersion = 2

// maxSnapshotBytes 限制快照解压后的大小，防止异常输入耗尽内存。
const maxSnapshotBytes = 32 << 20

var (
	// ErrCatalogEmpty 表示目录中没有任何可用的推理模型条目。
	ErrCatalogEmpty = errors.New("model catalog is empty")
	// ErrCatalogMalformed 表示目录或快照不是预期的 JSON 结构。
	ErrCatalogMalformed = errors.New("model catalog is malformed")
	// ErrSnapshotOutdated 表示快照来自旧格式版本；调用方应视同缺失，用内置快照重新写入。
	ErrSnapshotOutdated = errors.New("model catalog snapshot is outdated")
)

type modelsDevReasoningModel struct {
	ID               string                     `json:"id"`
	CanonicalModelID string                     `json:"canonical_model_id"`
	Reasoning        bool                       `json:"reasoning"`
	ReasoningOptions []modelsDevReasoningOption `json:"reasoning_options"`
	Modalities       struct {
		Input  []string `json:"input"`
		Output []string `json:"output"`
	} `json:"modalities"`
	Limit struct {
		Context float64 `json:"context"`
	} `json:"limit"`
}

type modelsDevReasoningOption struct {
	Type   string   `json:"type"`
	Values []any    `json:"values"`
	Min    *float64 `json:"min"`
	Max    *float64 `json:"max"`
}

// ParseCatalog 把 models.dev api.json 精简为目录条目，保留以下任一信息：
//   - 推理能力：reasoning 为 true 且带有可识别的 reasoning_options；
//   - 输入模态：modalities.input 中的已知模态（含纯文本模型，用于判断「不支持图片」）。
//
// 保留下来的条目同时带上输出模态与 limit.context，供管理端展示与自动填充上下文窗口。
//
// 单个 provider 或模型结构异常时跳过，不影响其余条目；结果按 provider 与模型 id 排序，保证快照内容稳定。
func ParseCatalog(raw []byte) ([]domainchannel.ModelCatalogEntry, error) {
	var providers map[string]json.RawMessage
	if err := json.Unmarshal(raw, &providers); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCatalogMalformed, err)
	}
	entries := make([]domainchannel.ModelCatalogEntry, 0, 4096)
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
			if json.Unmarshal(modelRaw, &model) != nil {
				continue
			}
			modelID := strings.TrimSpace(model.ID)
			if modelID == "" {
				modelID = strings.TrimSpace(modelKey)
			}
			var options []domainchannel.ReasoningCatalogOption
			if model.Reasoning {
				options = normalizeModelsDevReasoningOptions(model.ReasoningOptions)
			}
			inputs := domainchannel.NormalizeInputModalities(model.Modalities.Input)
			if modelID == "" || (len(options) == 0 && len(inputs) == 0) {
				continue
			}
			entries = append(entries, domainchannel.ModelCatalogEntry{
				Provider:         provider,
				ModelID:          modelID,
				CanonicalID:      strings.TrimSpace(model.CanonicalModelID),
				Reasoning:        len(options) > 0,
				Options:          options,
				InputModalities:  inputs,
				OutputModalities: domainchannel.NormalizeOutputModalities(model.Modalities.Output),
				ContextWindow:    domainchannel.NormalizeCatalogContextWindow(int(model.Limit.Context)),
			})
		}
	}
	if len(entries) == 0 {
		return nil, ErrCatalogEmpty
	}
	sortCatalogEntries(entries)
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

func sortCatalogEntries(entries []domainchannel.ModelCatalogEntry) {
	sort.SliceStable(entries, func(left, right int) bool {
		if entries[left].Provider != entries[right].Provider {
			return entries[left].Provider < entries[right].Provider
		}
		return entries[left].ModelID < entries[right].ModelID
	})
}

// catalogSnapshotFile 是快照的序列化结构，storage 缓存（明文）与内置快照（gzip）共用。
type catalogSnapshotFile struct {
	Version   int                    `json:"version"`
	Source    string                 `json:"source"`
	Origin    string                 `json:"origin,omitempty"`
	FetchedAt time.Time              `json:"fetchedAt"`
	Models    []catalogSnapshotModel `json:"models"`
}

type catalogSnapshotModel struct {
	Provider    string                  `json:"provider"`
	ID          string                  `json:"id"`
	CanonicalID string                  `json:"canonicalId,omitempty"`
	Options     []catalogSnapshotOption `json:"options,omitempty"`
	Inputs      []string                `json:"inputs,omitempty"`
	Outputs     []string                `json:"outputs,omitempty"`
	Context     int                     `json:"context,omitempty"`
}

type catalogSnapshotOption struct {
	Type   string   `json:"type"`
	Values []string `json:"values,omitempty"`
	Min    *int     `json:"min,omitempty"`
	Max    *int     `json:"max,omitempty"`
}

// MarshalSnapshot 把快照序列化为缩进的明文 JSON，用于 storage 缓存，便于运维查看。
func MarshalSnapshot(snapshot domainchannel.ModelCatalogSnapshot) ([]byte, error) {
	file, err := newSnapshotFile(snapshot)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(file, "", "  ")
}

// EncodeSnapshot 把快照序列化为 gzip 压缩的 JSON，用于内置快照；gzip 头不带时间戳，相同输入产出相同字节。
func EncodeSnapshot(snapshot domainchannel.ModelCatalogSnapshot) ([]byte, error) {
	file, err := newSnapshotFile(snapshot)
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

func newSnapshotFile(snapshot domainchannel.ModelCatalogSnapshot) (catalogSnapshotFile, error) {
	if len(snapshot.Entries) == 0 {
		return catalogSnapshotFile{}, ErrCatalogEmpty
	}
	file := catalogSnapshotFile{
		Version:   snapshotVersion,
		Source:    strings.TrimSpace(snapshot.Source),
		Origin:    strings.TrimSpace(snapshot.Origin),
		FetchedAt: snapshot.FetchedAt.UTC(),
		Models:    make([]catalogSnapshotModel, 0, len(snapshot.Entries)),
	}
	for _, entry := range snapshot.Entries {
		model := catalogSnapshotModel{
			Provider:    entry.Provider,
			ID:          entry.ModelID,
			CanonicalID: entry.CanonicalID,
			Inputs:      entry.InputModalities,
			Outputs:     entry.OutputModalities,
			Context:     entry.ContextWindow,
		}
		for _, option := range entry.Options {
			model.Options = append(model.Options, catalogSnapshotOption{
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

// DecodeSnapshot 解析 EncodeSnapshot（gzip）或 MarshalSnapshot（明文）的输出。
// 版本不符、结构异常或没有条目时返回错误，调用方应回退到其他来源。
func DecodeSnapshot(data []byte) (domainchannel.ModelCatalogSnapshot, error) {
	var reader io.Reader = bytes.NewReader(data)
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzipReader, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrCatalogMalformed, err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}
	plain, err := io.ReadAll(io.LimitReader(reader, maxSnapshotBytes+1))
	if err != nil {
		return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrCatalogMalformed, err)
	}
	if len(plain) > maxSnapshotBytes {
		return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: snapshot exceeds %d bytes", ErrCatalogMalformed, maxSnapshotBytes)
	}
	var file catalogSnapshotFile
	if err := json.Unmarshal(plain, &file); err != nil {
		return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: %v", ErrCatalogMalformed, err)
	}
	if file.Version > 0 && file.Version < snapshotVersion {
		return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: snapshot version %d", ErrSnapshotOutdated, file.Version)
	}
	if file.Version != snapshotVersion {
		return domainchannel.ModelCatalogSnapshot{}, fmt.Errorf("%w: unsupported snapshot version %d", ErrCatalogMalformed, file.Version)
	}
	snapshot := domainchannel.ModelCatalogSnapshot{
		Source:    file.Source,
		Origin:    file.Origin,
		FetchedAt: file.FetchedAt,
		Entries:   make([]domainchannel.ModelCatalogEntry, 0, len(file.Models)),
	}
	for _, model := range file.Models {
		inputs := domainchannel.NormalizeInputModalities(model.Inputs)
		if strings.TrimSpace(model.Provider) == "" || strings.TrimSpace(model.ID) == "" || (len(model.Options) == 0 && len(inputs) == 0) {
			continue
		}
		entry := domainchannel.ModelCatalogEntry{
			Provider:         model.Provider,
			ModelID:          model.ID,
			CanonicalID:      model.CanonicalID,
			Reasoning:        len(model.Options) > 0,
			InputModalities:  inputs,
			OutputModalities: domainchannel.NormalizeOutputModalities(model.Outputs),
			ContextWindow:    domainchannel.NormalizeCatalogContextWindow(model.Context),
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
		return domainchannel.ModelCatalogSnapshot{}, ErrCatalogEmpty
	}
	return snapshot, nil
}
