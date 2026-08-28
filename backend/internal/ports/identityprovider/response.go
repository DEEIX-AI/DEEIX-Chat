// Package identityprovider 定义身份源出站调用端口的数据契约。
package identityprovider

import "net/http"

// Response 是身份源 HTTP 调用经过边界化处理后的响应。
type Response struct {
	StatusCode int
	Status     string
	Body       []byte
}

// Successful 判断身份源是否返回 2xx 状态码。
func (r Response) Successful() bool {
	return r.StatusCode >= http.StatusOK && r.StatusCode < http.StatusMultipleChoices
}
