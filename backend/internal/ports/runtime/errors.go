// Package runtime 定义应用服务与受管运行时依赖的
// 基础设施适配器之间共享的契约。
package runtime

import "errors"

// ErrContainerNotFound 表示请求的受管容器不存在。
// Docker 适配器负责将引擎特定的退出输出
// 转换为该稳定的基础设施错误。
var ErrContainerNotFound = errors.New("runtime container not found")
