# 自定义品牌资源

> **许可与商标说明**
>
> DEEIX Chat Copyright 2026 DEEIX。源代码依据 [Apache License 2.0](../LICENSE) 授权，品牌配置不会移除或替换仓库及分发产物中的 [NOTICE](../NOTICE)、许可证副本或版权声明。重新分发本项目或其衍生作品时，须遵守 Apache License 2.0 并保留适用的归因信息。Apache License 2.0 不授予 DEEIX 名称、Logo 或其他商标的使用权，但合理描述作品来源及复制 NOTICE 内容的情形除外。

DEEIX Chat 的标题、HTML Meta Description、LOGO、浏览器图标和 PWA 图标通过前端构建环境变量配置。配置值会在 `pnpm build` 或 Docker 镜像构建时写入静态产物；修改运行中容器的环境变量不会更新已经构建好的页面。

所有变量都可独立配置。未配置或只包含空白字符时，对应位置继续使用仓库内置资源。

品牌配置只影响明确允许定制的产品展示区域。用户端和管理端的“关于”页面始终展示 DEEIX 官方 Logo、产品介绍、版权、官网、仓库和许可证信息，不读取任何品牌环境变量。批量文案替换不会处理管理端专用文案。公开分享页和聊天截图可以展示自定义品牌；配置自定义 Logo 时，同时会显示固定的 `Powered by DEEIX` 来源标识。

## 资源与环境变量

| 环境变量 | 使用位置 | 推荐格式与尺寸 | 内置资源 |
| --- | --- | --- | --- |
| `NEXT_PUBLIC_BRAND_TITLE` | HTML 标题、PWA 名称及前端默认产品名称 | 简短的纯文本产品名称 | `DEEIX Chat` |
| `NEXT_PUBLIC_BRAND_SHORT_NAME` | PWA 短名称、生成占位动画和 Artifact 标识 | 推荐不超过 12 个字符 | `DEEIX` |
| `NEXT_PUBLIC_BRAND_DESCRIPTION` | 仅 HTML `<head>` 中的 `<meta name="description">` | 简洁的纯文本页面摘要 | `DEEIX Chat is a multi-model AI conversation system.` |
| `NEXT_PUBLIC_LOGO_URL` | 登录页、侧边栏、移动端标题、公开分享页和聊天截图的产品品牌区域 | SVG 优先；PNG/WebP 至少 `1200 × 369`，推荐约 `3.25:1` | 浅色主题使用 `frontend/public/logo.svg`，深色主题使用 `frontend/public/logo-white.svg` |
| `NEXT_PUBLIC_FAVICON_URL` | 浏览器标签页、书签 | SVG 或包含 `16/32/48/64/128/256 px` 多尺寸的 ICO；也可使用至少 `64 × 64` 的 PNG | `frontend/public/favicon.ico` |
| `NEXT_PUBLIC_PWA_ICON_192_URL` | PWA manifest、浏览器通知 | `192 × 192` PNG | `frontend/public/pwa/icon-192.png` |
| `NEXT_PUBLIC_PWA_ICON_512_URL` | PWA manifest、高分辨率安装图标 | `512 × 512` PNG | `frontend/public/pwa/icon-512.png` |
| `NEXT_PUBLIC_PWA_MASKABLE_ICON_512_URL` | Android 等平台的自适应 PWA 图标 | `512 × 512` PNG，必须包含铺满画布的不透明背景 | `frontend/public/pwa/icon-maskable-512.png` |
| `NEXT_PUBLIC_APPLE_TOUCH_ICON_180_URL` | iOS/iPadOS 主屏幕图标 | `180 × 180` PNG，推荐使用铺满画布的不透明背景 | `frontend/public/pwa/apple-touch-icon.png` |

`NEXT_PUBLIC_LOGO_URL` 配置后，浅色与深色主题会使用同一张自定义图片。请确保它在白色背景和深色背景（可用 `#0f172a` 检查）上都有足够对比度。若使用单色字标，建议增加有对比度的底板、描边或选择同时适配两种背景的品牌色。

`NEXT_PUBLIC_BRAND_DESCRIPTION` 只控制搜索引擎等读取的 HTML Meta Description。PWA manifest 的 description、用户端和管理端“关于”页描述不会被它覆盖。

## 不参与定制的 DEEIX 信息

以下内容不受品牌环境变量影响：

- 用户端和管理端“关于”页面中的 DEEIX Logo、产品介绍、版权、官网、仓库、社交账号和许可证信息。
- 配置自定义 Logo 时，公开分享页右下角和聊天截图顶部右侧的 `Powered by DEEIX` 来源标识，以及底部固定的 DEEIX Logo。
- 仓库及 Docker 镜像中的 `LICENSE` 和 `NOTICE` 文件。

这些限制只描述仓库内置品牌配置的行为，不改变前述许可证授权与商标边界。

## LOGO 尺寸与留白

仓库当前字标可以作为横向 LOGO 的比例参考：

- `frontend/public/logo.svg` 的画布约为 `2400 × 737`，宽高比约 `3.25:1`。
- SVG 的可见图形约占画布宽度的 `98%`、高度的 `94%`，左右透明边约各 `1%`，上下透明边约各 `3%`。这是偏紧凑的下限，不建议裁切得更紧。
- 页面实际显示高度主要在 `20–56 px` 之间，因此细线、小字号标语和复杂纹理在界面中容易丢失。

自定义横向 LOGO 推荐遵循以下规则：

1. 优先提供带正确 `viewBox` 的 SVG；如果使用位图，推荐 `1200 × 369` 或更高的同等比例图片。
2. 画布宽高比保持在 `3:1–3.5:1`，推荐 `3.25:1`。不要使用正方形应用图标代替横向字标。
3. 可见图形距离左右边界至少保留画布宽度的 `2%`，距离上下边界至少保留画布高度的 `4%`；带阴影、外发光或描边时，推荐分别提高到 `4%` 和 `6%`。
4. 所有留白应位于图片画布内部并保持透明，不要通过写死白色背景制造留白。
5. 在 `20 px` 高度下检查辨识度，在 `56 px` 高度下检查边缘、透明区和视觉居中。

以 `1200 × 369` 画布为例，最低透明留白约为：

- 左右各 `24 px`；带外部效果时推荐各 `48 px`。
- 上下各 `15 px`；带外部效果时推荐各 `22 px`。

## 方形图标与安全区

普通 PWA 图标、favicon 和 Apple Touch 图标应使用同一个方形品牌符号，不要直接缩小完整横向字标。

### 普通图标

仓库当前普通图标的可见符号占比可作为示例：

- `icon-192.png` 的可见区域约为 `148 × 150`，四周留白约 `11%`。
- `icon-512.png` 的可见区域约为 `392 × 400`，四周留白约 `11%`。
- `apple-touch-icon.png` 的可见区域约为 `138 × 142`，四周留白约 `11%`。
- `favicon.ico` 内含 `16/32/48/64/128/256 px` 六档图片；其中 `256 px` 图片的可见区域约为 `196 × 200`，四周同样保留约 `11%`。

推荐让核心符号占画布宽、高的 `76%–80%`，即四边各保留约 `10%–12%` 的视觉留白：

| 画布 | 推荐核心符号宽、高 | 推荐单边留白 |
| --- | --- | --- |
| `192 × 192` | 各 `146–154 px` | `19–23 px` |
| `512 × 512` | 各 `389–410 px` | `51–62 px` |
| `180 × 180` | 各 `137–144 px` | `18–22 px` |

视觉重心不对称时，可以在上述范围内做少量光学校正，但不要让任何关键笔画进入最外侧 `8%` 的区域。

### Maskable 图标

Maskable 图标会被系统裁成圆形、圆角矩形或其他形状，必须为它单独导出资源：

1. 使用 `512 × 512` PNG，背景色或背景图形必须延伸到画布四边，不允许透明边缘。
2. 所有不可被裁掉的内容必须位于画布中央、直径为画布 `80%` 的圆形安全区内。
3. 为兼容方形符号的四角，推荐把核心符号控制在约 `56% × 56%` 以内，即最大约 `287 × 287 px`。
4. 仓库当前 `icon-maskable-512.png` 的核心符号约为 `274 × 280`，距离四边约 `23%`，可作为安全留白示例。
5. 背景可以超出安全区并铺满画布，但文字、徽标主体和关键边缘不能依赖安全区之外的内容。

## 配置示例

### 使用 HTTPS 资源

在 `frontend/.env.local` 中配置浏览器可访问的绝对 HTTPS URL，而不是文件系统路径：

```env
NEXT_PUBLIC_BRAND_TITLE="Example Chat"
NEXT_PUBLIC_BRAND_SHORT_NAME="Example"
NEXT_PUBLIC_BRAND_DESCRIPTION="Example Chat is a multi-model AI conversation system."
NEXT_PUBLIC_LOGO_URL=https://example.com/logo.png
NEXT_PUBLIC_FAVICON_URL=https://example.com/favicon.ico
NEXT_PUBLIC_PWA_ICON_192_URL=https://example.com/icon-192.png
NEXT_PUBLIC_PWA_ICON_512_URL=https://example.com/icon-512.png
NEXT_PUBLIC_PWA_MASKABLE_ICON_512_URL=https://example.com/icon-maskable-512.png
NEXT_PUBLIC_APPLE_TOUCH_ICON_180_URL=https://example.com/apple-touch-icon-180.png
```

承载 LOGO 的服务应返回允许当前站点读取图片的 CORS 响应头，否则聊天截图可能无法嵌入该图片。PWA 图标 URL 也应长期保持可公开访问。

### Docker 构建

Docker 镜像中的前端也是构建期配置，可直接传入绝对 HTTPS URL：

```bash
docker build \
  --build-arg NEXT_PUBLIC_BRAND_TITLE="Example Chat" \
  --build-arg NEXT_PUBLIC_BRAND_SHORT_NAME="Example" \
  --build-arg NEXT_PUBLIC_BRAND_DESCRIPTION="Example Chat is a multi-model AI conversation system." \
  --build-arg NEXT_PUBLIC_LOGO_URL=https://example.com/logo.png \
  --build-arg NEXT_PUBLIC_FAVICON_URL=https://example.com/favicon.ico \
  --build-arg NEXT_PUBLIC_PWA_ICON_192_URL=https://example.com/icon-192.png \
  --build-arg NEXT_PUBLIC_PWA_ICON_512_URL=https://example.com/icon-512.png \
  --build-arg NEXT_PUBLIC_PWA_MASKABLE_ICON_512_URL=https://example.com/icon-maskable-512.png \
  --build-arg NEXT_PUBLIC_APPLE_TOUCH_ICON_180_URL=https://example.com/apple-touch-icon-180.png \
  -t deeix-chat:branded .
```

生产环境推荐使用带版本号的 URL，例如 `https://example.com/v2/logo.png`。如果在同一 URL 下直接替换图片，浏览器、CDN 或已安装 PWA 可能继续显示旧缓存。

构建出的 Docker 镜像会在 `/app/licenses/DEEIX-Chat/` 中携带 `LICENSE` 和 `NOTICE`。重新打包或分发镜像时必须保留这两份文件。

## 构建后检查

完成配置后重新构建前端或 Docker 镜像，并至少检查：

- 浅色和深色主题中的登录页、侧边栏、移动端标题和公开分享页。
- 用户端和管理端“关于”页面仍完整展示 DEEIX 官方信息，没有被自定义品牌覆盖。
- 聊天截图顶部左侧显示产品品牌；配置自定义 Logo 时，右侧显示固定的 `Powered by DEEIX`。底部在配置自定义 Logo 时居中显示“自定义 Logo｜DEEIX Logo”，未配置时只显示 DEEIX Logo。
- 公开分享页顶部左侧显示产品品牌；配置自定义 Logo 时，视口右下角显示固定的 `Powered by DEEIX`。底部在配置自定义 Logo 时居中显示“自定义 Logo｜DEEIX Logo”，未配置时只显示 DEEIX Logo。
- 浏览器标签页和书签图标。
- HTML `<title>` 和 `<meta name="description">`，以及搜索引擎重新抓取后的标题与摘要。
- `manifest.webmanifest` 中的品牌名称、短名称、原始 description，以及 `192 × 192`、`512 × 512` 和 maskable 图标 URL。
- Chrome/Edge 安装后的 PWA 图标，以及 iOS/iPadOS“添加到主屏幕”图标。
- 后台回复完成通知所使用的图标。

如果只有 PWA 图标未更新，先确认新构建已经发布，再清理站点数据或卸载旧 PWA 后重新安装。生产环境应优先通过版本化 URL 更新品牌资源，避免依赖用户手动清理缓存。
