package conversation

import apppersonalprovider "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/personalprovider"

// 组合路由（个人模型 + 平台）替代平台路由注入会话服务。会话服务会按类型断言可选能力，
// 包装层漏实现任一接口都会静默关闭对应功能（如内部文本任务的默认路由兜底），因此在编译期锁定。
var (
	_ routeResolver              = (*apppersonalprovider.RouteResolver)(nil)
	_ defaultRouteResolver       = (*apppersonalprovider.RouteResolver)(nil)
	_ activeModelCatalogResolver = (*apppersonalprovider.RouteResolver)(nil)
)
