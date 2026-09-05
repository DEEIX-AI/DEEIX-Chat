// Package contentmoderation 定义内容审核出站调用端口的数据契约。
// 请求与响应类型位于 domain/contentmoderation；这里只声明适配器需要返回、消费方需要识别的错误语义。
package contentmoderation

import "errors"

var (
	// ErrInvalidBaseURL 表示审核服务地址不合法或不被出站策略允许。
	ErrInvalidBaseURL = errors.New("invalid content moderation base url")
	// ErrTimeout 表示审核服务在配置的超时时间内未返回。
	ErrTimeout = errors.New("content moderation timed out")
	// ErrService 表示审核服务返回了非成功状态。
	ErrService = errors.New("content moderation service error")
	// ErrRateLimited 表示审核服务返回限流。
	ErrRateLimited = errors.New("content moderation rate limited")
	// ErrInvalidResponse 表示审核服务响应无法解析或缺少必要字段。
	ErrInvalidResponse = errors.New("content moderation invalid response")
	// ErrNetwork 表示到审核服务的网络传输失败。
	ErrNetwork = errors.New("content moderation network error")
)
