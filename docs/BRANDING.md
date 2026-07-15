# 自定义品牌资源

> **许可与商标说明**
>
> DEEIX Chat Copyright 2026 DEEIX。源代码依据 [Apache License 2.0](../LICENSE) 授权。品牌配置不会移除或替换仓库及分发产物中的 [NOTICE](../NOTICE)、许可证副本或版权声明。Apache License 2.0 不授予 DEEIX 名称、Logo 或其他商标的使用权。

DEEIX Chat 的产品标题、HTML Meta Description、Logo、浏览器图标和 PWA 图标由根目录 `config.yaml` 统一配置。后端启动时读取配置并通过公开品牌接口提供给静态前端，因此修改品牌不需要重新构建前端或 Docker 镜像，只需更新配置并重启应用。

未配置或配置为空时，对应位置使用仓库内置品牌资源。品牌接口不可用时，前端也会保留内置品牌，不阻塞页面加载。

品牌配置只影响允许定制的产品展示区域。用户端和管理端的“关于”页面始终展示 DEEIX 官方 Logo、产品介绍、版权、官网、仓库和许可证信息。配置自定义 Logo 时，公开分享页和聊天截图仍会显示固定的 `Powered by DEEIX` 来源标识。

## 配置项

前端产品品牌统一使用 `branding`。`app.name` 仍是后端服务名称，不会因前端品牌定制而改变：

```yaml
branding:
  title: Example Chat
  short_name: Example
  description: Example Chat is a multi-model AI conversation system.
  logo_url: https://example.com/logo.svg
  favicon_url: https://example.com/favicon.ico
  pwa_icon_192_url: https://example.com/icon-192.png
  pwa_icon_512_url: https://example.com/icon-512.png
  pwa_maskable_icon_512_url: https://example.com/icon-maskable-512.png
  apple_touch_icon_180_url: https://example.com/apple-touch-icon-180.png
```

| YAML 配置 | 使用位置 | 推荐格式与尺寸 | 内置资源 |
| --- | --- | --- | --- |
| `branding.title` | HTML 标题、PWA 名称及前端默认产品名称 | 简短的纯文本产品名称 | `DEEIX Chat` |
| `branding.short_name` | PWA 短名称、生成占位动画和 Artifact 标识 | 推荐不超过 12 个字符 | `DEEIX` |
| `branding.description` | HTML Meta Description 和 PWA description | 简洁的纯文本页面摘要 | `DEEIX Chat is a multi-model AI conversation system.` |
| `branding.logo_url` | 登录页、侧边栏、移动端标题、公开分享页和聊天截图 | SVG 优先；PNG/WebP 至少 `1200 x 369`，推荐约 `3.25:1` | 浅色使用 `frontend/public/logo.svg`，深色使用 `frontend/public/logo-white.svg` |
| `branding.favicon_url` | 浏览器标签页、书签 | SVG、ICO，或至少 `64 x 64` 的 PNG | `frontend/public/favicon.ico` |
| `branding.pwa_icon_192_url` | PWA manifest、浏览器通知 | `192 x 192` PNG | `frontend/public/pwa/icon-192.png` |
| `branding.pwa_icon_512_url` | PWA manifest、高分辨率安装图标 | `512 x 512` PNG | `frontend/public/pwa/icon-512.png` |
| `branding.pwa_maskable_icon_512_url` | Android 等平台的自适应 PWA 图标 | `512 x 512` PNG，背景铺满画布 | `frontend/public/pwa/icon-maskable-512.png` |
| `branding.apple_touch_icon_180_url` | iOS/iPadOS 主屏幕图标 | `180 x 180` PNG，背景铺满画布 | `frontend/public/pwa/apple-touch-icon.png` |

品牌资源可以使用浏览器可访问的绝对 HTTPS URL，也可以使用当前前端站点下的根相对路径。前后端分离部署时，推荐使用绝对 HTTPS URL，并正确设置 `server.public_web_base_url`，确保 Web App Manifest 的启动地址、作用域和内置图标指向前端站点。

`branding.logo_url` 配置后，浅色与深色主题使用同一张自定义图片。请确保图片在白色和深色背景上都有足够对比度。

承载 Logo 的服务需要允许当前站点读取图片，否则聊天截图可能无法嵌入该图片。图标 URL 应长期公开可访问；更新资源时推荐使用带版本号的 URL，避免浏览器或已安装 PWA 继续使用旧缓存。

## 不参与定制的 DEEIX 信息

以下内容不受品牌配置影响：

- 用户端和管理端“关于”页面中的 DEEIX Logo、产品介绍、版权、官网、仓库、社交账号和许可证信息。
- 自定义 Logo 场景下的 `Powered by DEEIX` 来源标识和固定 DEEIX Logo。
- 仓库及 Docker 镜像中的 `LICENSE` 和 `NOTICE` 文件。

这些限制只描述仓库内置品牌配置的行为，不改变许可证授权与商标边界。

## Logo 尺寸与留白

仓库字标画布约为 `2400 x 737`，宽高比约 `3.25:1`。自定义横向 Logo 推荐：

1. 优先提供带正确 `viewBox` 的 SVG；位图推荐使用 `1200 x 369` 或更高的同等比例图片。
2. 画布宽高比保持在 `3:1-3.5:1`，不要使用正方形应用图标替代横向字标。
3. 可见图形距离左右边界至少保留画布宽度的 `2%`，距离上下边界至少保留画布高度的 `4%`。
4. 留白应位于图片画布内部并保持透明，不要通过写死白色背景制造留白。
5. 在 `20 px` 和 `56 px` 显示高度下分别检查辨识度、边缘和视觉居中。

## 方形图标与安全区

普通 PWA 图标、favicon 和 Apple Touch 图标应使用方形品牌符号，不要直接缩小完整横向字标。

- 普通图标建议让核心符号占画布宽高的 `76%-80%`，四边保留约 `10%-12%` 的视觉留白。
- Maskable 图标必须使用 `512 x 512` PNG，背景延伸到画布四边。
- Maskable 图标的关键内容必须位于画布中央、直径为画布 `80%` 的圆形安全区内。
- 为兼容方形符号的四角，核心符号建议控制在约 `56% x 56%` 以内。

## 生效与检查

更新 `config.yaml` 后重启应用。静态页面水合时由根级品牌 Provider 请求一次 `/api/v1/branding`，品牌应用完成前页面不会显示，因此首个可见画面直接使用运行时品牌；接口异常时最多等待 3 秒便回退到内置品牌。Provider 在后续客户端路由切换中持续持有同一份标题、图标和文案，不会逐页请求或重新读取配置文件。

### 同源部署

Docker 镜像内的前端仍是 Next.js 静态导出文件，由同一个 Go 服务提供。品牌接口和 manifest 使用相对地址，浏览器不会产生跨域请求。后端同时在 `/manifest.webmanifest` 返回动态品牌 manifest，使使用旧根路径安装的 PWA 仍有机会按浏览器更新周期获取新配置。

### 分离部署

前端可以继续部署为纯静态文件，后端独立部署。构建前端时通过 `NEXT_PUBLIC_API_BASE_URL` 指定后端公开地址；该变量只用于定位 API，不包含任何品牌值。品牌仍在部署时由后端 `config.yaml` 提供，修改品牌不需要重新构建前端。

分离部署还需要：

- 将前端来源加入后端 `server.cors_allow_origin`。
- 将 `server.public_web_base_url` 设置为前端公开地址，使 manifest 的 `id`、`start_url` 和 `scope` 指向前端站点。
- 确保浏览器可访问 `${NEXT_PUBLIC_API_BASE_URL}/api/v1/branding` 和 `${NEXT_PUBLIC_API_BASE_URL}/api/v1/branding/manifest.webmanifest`。
- 若要让旧 PWA 沿用前端域名下的 `/manifest.webmanifest` 原地更新，可在前端网关将该路径代理到后端同名路由；无法配置代理时，旧安装可能需要卸载后重新安装。

至少检查以下位置：

- 浅色和深色主题中的登录页、侧边栏、移动端标题和公开分享页。
- 用户端和管理端“关于”页面仍完整展示 DEEIX 官方信息。
- 聊天截图和公开分享页中的自定义品牌及 DEEIX 来源标识。
- 浏览器标题、Meta Description、favicon 和 Apple Touch Icon。
- `/api/v1/branding`、`/api/v1/branding/manifest.webmanifest` 和同源兼容地址 `/manifest.webmanifest` 中的品牌名称、启动地址和图标。
- 后台回复完成通知使用的品牌名称和图标。

普通页面刷新后会立即使用新品牌。已经安装的 PWA 名称和应用图标由浏览器独立缓存并按浏览器自己的周期检查 manifest，不保证在应用重启后立刻更新。生产环境应使用版本化图标 URL；需要立即验证时，请卸载旧 PWA、清理该站点的应用数据后重新安装。
