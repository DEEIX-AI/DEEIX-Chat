package billing

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// OpenRouter 官方定价数据来源。
const (
	OfficialPricingOriginRemote  = "remote"
	OfficialPricingOriginBuiltin = "builtin"
)

// maxOpenRouterPricingSnapshotBytes 限制内置快照解压后的大小，防止异常输入耗尽内存。
const maxOpenRouterPricingSnapshotBytes = 32 << 20

// EncodeOpenRouterPricingSnapshot 拉取 OpenRouter 官方定价，按运行时缓存的同一格式规范化后
// 输出 gzip JSON，作为随版本发布的内置快照（见 catalogdata）。gzip 头不带时间戳，相同输入产出相同字节。
func EncodeOpenRouterPricingSnapshot(ctx context.Context, provider OpenRouterPricingProvider, fetchedAt time.Time) ([]byte, int, error) {
	items, err := (&OfficialPricingService{provider: provider}).FetchOpenRouterOfficialPricing(ctx)
	if err != nil {
		return nil, 0, err
	}
	file := openRouterPricingCacheFile{
		Version:   openRouterPricingCacheVersion,
		FetchedAt: fetchedAt.UTC(),
		Origin:    OfficialPricingOriginBuiltin,
		Items:     officialPricingCacheItems(items),
	}
	var buffer bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	if err != nil {
		return nil, 0, err
	}
	if err := json.NewEncoder(writer).Encode(file); err != nil {
		return nil, 0, err
	}
	if err := writer.Close(); err != nil {
		return nil, 0, err
	}
	return buffer.Bytes(), len(file.Items), nil
}

// SetBuiltinSnapshot 注入内置定价快照（EncodeOpenRouterPricingSnapshot 的输出）。
// 快照无效或格式过旧时返回错误并忽略，服务照常只依赖远端与持久缓存。
func (s *OfficialPricingService) SetBuiltinSnapshot(data []byte) error {
	if s == nil {
		return nil
	}
	snapshot, err := decodeOpenRouterPricingSnapshot(data)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builtin = &snapshot
	return nil
}

func decodeOpenRouterPricingSnapshot(data []byte) (openRouterPricingCacheFile, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return openRouterPricingCacheFile{}, fmt.Errorf("decode builtin openrouter pricing: %w", err)
	}
	defer reader.Close()
	plain, err := io.ReadAll(io.LimitReader(reader, maxOpenRouterPricingSnapshotBytes+1))
	if err != nil {
		return openRouterPricingCacheFile{}, fmt.Errorf("decode builtin openrouter pricing: %w", err)
	}
	if len(plain) > maxOpenRouterPricingSnapshotBytes {
		return openRouterPricingCacheFile{}, fmt.Errorf("builtin openrouter pricing exceeds %d bytes", maxOpenRouterPricingSnapshotBytes)
	}
	snapshot, ok := decodeOpenRouterPricingCacheFile(plain)
	if !ok || snapshot.Version < openRouterPricingCacheVersion {
		return openRouterPricingCacheFile{}, fmt.Errorf("builtin openrouter pricing is empty or outdated")
	}
	snapshot.Origin = OfficialPricingOriginBuiltin
	return snapshot, nil
}
