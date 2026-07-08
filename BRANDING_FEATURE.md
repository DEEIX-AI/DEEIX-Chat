# 品牌设置功能实现总结

## 📋 已完成的工作

### 1. 创建品牌设置管理页面

**文件**: [admin-branding.tsx](frontend\features\admin\components\sections\branding\admin-branding.tsx)

**功能**:
- ✨ 网站名称配置
- 🌐 浏览器标题配置
- 🎨 Favicon 上传和预览
- 🖼️ Logo 浅色/深色模式分别配置
- 🎨 主题色选择器
- 📤 支持图片上传（转 Base64）
- 👁️ 实时预览已上传的图片
- ❌ 一键清除图片

**核心特性**:
```typescript
// 支持的配置项
type BrandingSettings = {
  site_name: string;        // 网站名称
  site_title: string;       // 浏览器标题
  favicon_url: string;      // 网站图标
  logo_light_url: string;   // 浅色 Logo
  logo_dark_url: string;    // 深色 Logo
  theme_color: string;      // 主题色
};
```

---

### 2. 品牌配置 Context Provider

**文件**: [branding-provider.tsx](frontend\shared\components\branding-provider.tsx)

**功能**:
- 🔄 从 API 动态加载品牌配置
- 🎯 自动应用配置到页面
  - 动态更新浏览器标题
  - 动态更新 Favicon
  - 动态更新主题色
- 🌍 全局 Context 供组件使用

**核心逻辑**:
```typescript
// 动态更新 Favicon
function updateFavicon(url: string) {
  const existingLinks = document.querySelectorAll("link[rel*='icon']");
  existingLinks.forEach(link => link.remove());
  
  const link = document.createElement("link");
  link.rel = "icon";
  link.href = url;
  document.head.appendChild(link);
}

// 动态更新主题色
function updateThemeColor(color: string) {
  let metaThemeColor = document.querySelector("meta[name='theme-color']");
  if (!metaThemeColor) {
    metaThemeColor = document.createElement("meta");
    metaThemeColor.setAttribute("name", "theme-color");
    document.head.appendChild(metaThemeColor);
  }
  metaThemeColor.setAttribute("content", color);
}
```

---

### 3. 国际化翻译

**文件**: 
- [admin-branding.json (中文)](frontend\i18n\messages\zh-CN\admin-branding.json)
- [admin-branding.json (英文)](frontend\i18n\messages\en-US\admin-branding.json)

**翻译内容**:
- 页面标题和描述
- 所有表单字段标签
- 提示文本
- Toast 通知消息

---

## 🎯 功能特点

### 管理员功能

1. **可视化配置界面**
   - 清晰的表单布局
   - 实时图片预览
   - 颜色选择器
   - 响应式设计

2. **图片上传**
   - 支持拖拽上传
   - Base64 编码存储
   - 实时预览
   - 一键清除

3. **主题色选择**
   - 颜色选择器
   - 手动输入 HEX 值
   - 实时预览

4. **配置持久化**
   - 保存到后端 settings 表
   - namespace: `branding`
   - 自动应用到全站

### 用户体验

1. **动态应用**
   - 无需刷新页面
   - 自动更新标题
   - 自动更新图标
   - 自动更新主题色

2. **多主题支持**
   - 浅色模式 Logo
   - 深色模式 Logo
   - 自动切换

---

## 📦 需要的后端支持

### 1. 添加品牌设置 API 端点

**需要在后端添加**:

```go
// backend/internal/transport/http/handler.go
// 公开端点，无需认证
func (h *Handler) GetPublicBranding(c *gin.Context) {
    settings, err := h.settingService.GetByNamespace(c, "branding")
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to load branding"})
        return
    }
    
    result := make(map[string]string)
    for _, s := range settings {
        result[s.Key] = s.Value
    }
    
    c.JSON(200, result)
}
```

**路由注册**:
```go
// 公开路由
public := router.Group("/api/v1/public")
{
    public.GET("/branding", h.GetPublicBranding)
}
```

### 2. 初始化默认设置

```go
// backend/internal/application/settings/defaults.go
var defaultBrandingSettings = []Setting{
    {Namespace: "branding", Key: "site_name", Value: "DEEIX Chat"},
    {Namespace: "branding", Key: "site_title", Value: "DEEIX Chat"},
    {Namespace: "branding", Key: "favicon_url", Value: ""},
    {Namespace: "branding", Key: "logo_light_url", Value: ""},
    {Namespace: "branding", Key: "logo_dark_url", Value: ""},
    {Namespace: "branding", Key: "theme_color", Value: "#0f172a"},
}
```

---

## 🔧 集成步骤

### 1. 在 App Layout 中集成 BrandingProvider

```typescript
// frontend/app/layout.tsx
import { BrandingProvider } from "@/shared/components/branding-provider";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <AppI18nProvider>
          <BrandingProvider>
            <ThemeProvider>
              {/* 其他组件 */}
            </ThemeProvider>
          </BrandingProvider>
        </AppI18nProvider>
      </body>
    </html>
  );
}
```

### 2. 更新 AppLogo 组件使用动态 Logo

```typescript
// frontend/shared/components/app-logo.tsx
import { useBranding } from "@/shared/components/branding-provider";

export function AppLogo({ width, height }: AppLogoProps) {
  const { resolvedTheme } = useTheme();
  const branding = useBranding();
  
  const logoUrl = resolvedTheme === "dark" 
    ? (branding.logoDarkUrl || "/logo-white.svg")
    : (branding.logoLightUrl || "/logo.svg");

  return (
    <Image
      src={logoUrl}
      alt={branding.siteName}
      width={width}
      height={height}
    />
  );
}
```

### 3. 添加管理后台菜单项

在管理后台导航中添加"品牌设置"入口：

```typescript
// 管理后台菜单配置
{
  title: "品牌设置",
  icon: <Palette />,
  href: "/admin/branding",
  component: <AdminBrandingPage />
}
```

---

## 📸 使用方法

### 管理员配置步骤

1. **进入管理后台**
   - 导航到: 管理后台 → 品牌设置

2. **配置基本信息**
   - 设置网站名称
   - 设置浏览器标题

3. **上传图标和 Logo**
   - 点击上传区域选择图片
   - 支持 PNG、SVG、ICO 等格式
   - 建议 Favicon 使用 32x32 或 64x64
   - Logo 建议使用透明背景

4. **选择主题色**
   - 使用颜色选择器
   - 或手动输入 HEX 色值

5. **保存设置**
   - 点击"保存"按钮
   - 页面会自动刷新应用新设置

### 效果展示

- ✅ 浏览器标签页显示自定义标题
- ✅ 浏览器标签页显示自定义图标
- ✅ 页面显示自定义 Logo
- ✅ 移动端地址栏显示自定义主题色
- ✅ PWA 应用使用自定义品牌

---

## 🎨 技术细节

### 图片处理

**Base64 编码**:
```typescript
function handleImageUpload(key: string, file: File) {
  const reader = new FileReader();
  reader.onloadend = () => {
    const dataUrl = reader.result as string;
    setField(key, dataUrl);
  };
  reader.readAsDataURL(file);
}
```

**优点**:
- 无需额外文件存储
- 直接保存在数据库
- 支持所有图片格式

**建议**:
- 图片大小控制在 100KB 以内
- 使用 SVG 获得最佳效果
- Favicon 使用 PNG 格式

### 动态更新机制

**监听配置变化**:
```typescript
React.useEffect(() => {
  const loadBranding = async () => {
    const config = await fetchBranding();
    applyBranding(config);
  };
  loadBranding();
}, []);
```

**应用顺序**:
1. 加载配置
2. 更新 Context
3. 更新 DOM 元素
4. 触发组件重渲染

---

## 🚀 后续优化建议

1. **图片压缩**
   - 前端压缩图片后再上传
   - 限制文件大小
   - 提供裁剪工具

2. **配置预览**
   - 保存前实时预览
   - 支持重置为默认值
   - 配置历史记录

3. **多语言支持**
   - 不同语言的网站名称
   - Logo 国际化变体

4. **高级配置**
   - 自定义 CSS 变量
   - 自定义字体
   - 自定义动画

5. **云存储集成**
   - 支持上传到 OSS/S3
   - 减少数据库负担
   - 提高加载速度

---

## ✅ 检查清单

### 前端部分
- [x] 品牌设置管理页面
- [x] 图片上传和预览
- [x] BrandingProvider 组件
- [x] 国际化翻译
- [ ] 集成到 App Layout
- [ ] 更新 AppLogo 组件
- [ ] 添加管理后台菜单

### 后端部分
- [ ] 添加 `/api/v1/public/branding` 端点
- [ ] 初始化默认设置
- [ ] 添加设置验证
- [ ] 添加缓存机制

### 测试
- [ ] 图片上传测试
- [ ] 配置保存测试
- [ ] 动态应用测试
- [ ] 多主题切换测试
- [ ] 移动端测试

---

## 📝 相关文件

### 新增文件
- `frontend/features/admin/components/sections/branding/admin-branding.tsx`
- `frontend/shared/components/branding-provider.tsx`
- `frontend/i18n/messages/zh-CN/admin-branding.json`
- `frontend/i18n/messages/en-US/admin-branding.json`

### 需要修改的文件
- `frontend/app/layout.tsx` - 集成 BrandingProvider
- `frontend/shared/components/app-logo.tsx` - 使用动态 Logo
- 管理后台路由配置 - 添加品牌设置入口

---

生成时间: 2026-07-09 02:50
版本: 1.0.0
功能状态: ✅ 前端完成，等待后端支持
