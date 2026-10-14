package uicomponent

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	appaudit "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/audit"
	domainuicomponent "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/uicomponent"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/pagination"
)

const (
	maxNameLength          = 64
	maxDescriptionLength   = 256
	maxPropsSummaryLength  = 1024
	maxPropsSchemaBytes    = 16 * 1024
	maxRendererSourceBytes = 256 * 1024
	maxCatalogComponents   = 32
)

// 名称同时是提示词里的标记和前端分发键，限制为 kebab-case 标识符。
var componentNamePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

// Service 封装交互式组件业务逻辑。
type Service struct {
	repo        repository.UIComponentRepository
	auditWriter auditWriter
	// featureEnabled 读取全局开关 chat.ui_components_enabled；为 nil 时视为开启。
	featureEnabled func() bool
}

type auditWriter interface {
	Write(ctx context.Context, input appaudit.WriteInput)
}

// NewService 创建组件服务。
func NewService(repo repository.UIComponentRepository) *Service {
	return &Service{repo: repo}
}

// SetAuditWriter 注入审计写入器。
func (s *Service) SetAuditWriter(writer auditWriter) {
	s.auditWriter = writer
}

// SetFeatureEnabled 注入全局开关读取函数。关闭时用户侧目录为空、会话解析不到任何组件，
// 客户端因此不再显示选择器也不渲染组件块；管理端列表不受影响，便于关着的时候仍能维护。
func (s *Service) SetFeatureEnabled(fn func() bool) {
	s.featureEnabled = fn
}

func (s *Service) enabled() bool {
	return s.featureEnabled == nil || s.featureEnabled()
}

// AuditInput 描述组件审计写入。
type AuditInput struct {
	UserID     uint
	RequestID  string
	Action     string
	ResourceID string
	ClientIP   string
	UserAgent  string
	Detail     any
}

// RecordAudit 记录组件审计日志。
func (s *Service) RecordAudit(ctx context.Context, input AuditInput) {
	if s.auditWriter == nil {
		return
	}
	s.auditWriter.Write(ctx, appaudit.WriteInput{
		RequestID:   input.RequestID,
		ActorUserID: input.UserID,
		Action:      input.Action,
		Resource:    "ui_components",
		ResourceID:  input.ResourceID,
		IP:          input.ClientIP,
		UserAgent:   input.UserAgent,
		Detail:      input.Detail,
	})
}

// ListInput 定义组件列表入参。
type ListInput struct {
	IDs      []uint
	Query    string
	Scope    string
	Enabled  *bool
	Page     int
	PageSize int
}

// WriteInput 定义组件创建入参。
type WriteInput struct {
	Name           string
	Version        int
	Description    string
	PropsSummary   string
	PropsSchema    string
	RendererSource string
	Enabled        bool
	SortOrder      int
}

// PatchInput 定义组件更新入参。
type PatchInput struct {
	Name           *string
	Version        *int
	Description    *string
	PropsSummary   *string
	PropsSchema    *string
	RendererSource *string
	Enabled        *bool
	SortOrder      *int
}

// ListVisible 查询当前用户可使用的组件（已启用的内置、平台组件与本人组件）。
func (s *Service) ListVisible(ctx context.Context, userID uint, input ListInput) ([]domainuicomponent.Component, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	if !s.enabled() {
		return []domainuicomponent.Component{}, 0, nil
	}
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{
		IDs:           input.IDs,
		Query:         strings.TrimSpace(input.Query),
		VisibleUserID: &userID,
	}, offset, limit)
}

// ListMine 查询当前用户自定义组件。
func (s *Service) ListMine(ctx context.Context, userID uint, input ListInput) ([]domainuicomponent.Component, int64, error) {
	if userID == 0 {
		return nil, 0, repository.ErrInvalidInput
	}
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	return s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{
		Query:       strings.TrimSpace(input.Query),
		Scope:       domainuicomponent.ScopeUser,
		OwnerUserID: &userID,
		Enabled:     input.Enabled,
	}, offset, limit)
}

// ListAdmin 查询管理员视角的内置与平台组件。
func (s *Service) ListAdmin(ctx context.Context, input ListInput) ([]domainuicomponent.Component, int64, error) {
	offset, limit := pagination.Offset(input.Page, input.PageSize)
	scope := strings.TrimSpace(input.Scope)
	if scope != "" && scope != domainuicomponent.ScopeBuiltin && scope != domainuicomponent.ScopePlatform {
		return nil, 0, ErrInvalidComponent
	}
	filter := repository.UIComponentListFilter{
		Query:   strings.TrimSpace(input.Query),
		Scope:   scope,
		Enabled: input.Enabled,
	}
	if scope == "" {
		zero := uint(0)
		filter.OwnerUserID = &zero
	}
	return s.repo.ListUIComponents(ctx, filter, offset, limit)
}

// ResolveVisible 返回当前用户可用且已启用的组件，按传入 ID 顺序去重；不可见的 ID 被忽略。
func (s *Service) ResolveVisible(ctx context.Context, userID uint, ids []uint) ([]domainuicomponent.Component, error) {
	if userID == 0 || len(ids) == 0 || !s.enabled() {
		return nil, nil
	}
	unique := uniqueIDs(ids)
	items, _, err := s.repo.ListUIComponents(ctx, repository.UIComponentListFilter{IDs: unique, VisibleUserID: &userID}, 0, len(unique))
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	byID := make(map[uint]domainuicomponent.Component, len(items))
	for _, item := range items {
		byID[item.ID] = item
	}
	ordered := make([]domainuicomponent.Component, 0, len(unique))
	for _, id := range unique {
		if item, ok := byID[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

// CatalogPrompt 生成给模型的组件目录说明。每个组件一行，渲染实现不进入提示词。
// catalogPromptHeader 是目录层的固定部分。结构化标签与视觉排版层保持同一风格；
// 最先说明组件是回复正文里的 Markdown 代码块、不是工具，避免模型把目录当成函数签名去调用。
const catalogPromptHeader = `<nature>交互式组件是回复正文里的一段 Markdown 代码块，由客户端渲染成界面。它是输出格式，不是工具：不要通过 function calling / tool call 调用组件，也不要等待它返回结果。</nature>
<when>只在组件明显比纯 Markdown 更清楚、更好用时使用（需要筛选、排序、计算、绘图、排期、逐步引导、对比、判分）；否则用 Markdown。数量由内容决定，可以为零。</when>
<syntax>语言标识为 ` + "`" + domainuicomponent.FenceLanguage + "`" + ` 的代码块，内容是合法 JSON，只有 component、id（简短英文、对话内唯一）、props 三个字段；字符串内的英文双引号写成 \"，不要尾随逗号、注释或目录外的字段。代码块放在正文中它所属的位置，前后用自然语言承接，不要在组件外重复组件里的数据。
` + "```" + domainuicomponent.FenceLanguage + `
{"component": "stat-grid", "id": "kpi-week", "props": {"title": "本周指标", "items": [{"label": "日活", "value": "128,430", "delta": "+4.2%", "trend": "up"}]}}
` + "```" + `</syntax>
<catalog>`

// 内置描述形如「定位。适用：…。交互：…」；交互说明是给选择器看的，提示词里只保留定位与适用场景。
const (
	propsRulesSeparator     = "。约定："
	descriptionInteractions = "。交互："
)

const catalogPromptFooter = `
</catalog>`

// liveExample 是可交互块的示例：房贷计算面板（state、computed、逐点公式与追问）。
//
//go:embed live_example.json
var liveExample string

// explorableExample 是「讲清一个关系」的示例：公式代入当前值、滑块、随状态变化的示意图。
//
//go:embed explorable_example.json
var explorableExample string

// liveCatalogHeader 生成可交互格式的目录层固定部分。公式语法、函数表与原语必须与 packages/core 的
// 表达式引擎一致；没勾选的 panel 或 widget，其条款与整节都不出现。
func liveCatalogHeader(panel, widget bool) string {
	sections := []string{livePromptIntro, liveUseRule(panel, widget), livePromptAvoid, liveChooseRule(panel, widget), livePromptRules}
	if panel {
		sections = append(sections, livePromptPanel)
	}
	if widget {
		sections = append(sections, livePromptWidget)
	}
	return strings.Join(sections, "\n")
}

const livePromptIntro = `<nature>交互式组件是回复正文里的一段 Markdown 代码块，由客户端渲染成界面。它是输出格式，不是工具：不要通过 function calling / tool call 调用组件，也不要等待它返回结果。</nature>
<when>
  <principle>组件是让用户直接操作、直接读数的，不是装饰。命中 <use> 时默认就该出组件，只把数据讲成一段话不算完成；没有任何命中时才用最轻的表达：文字 → Markdown 列表或表格。结论和要点写在正文，明细与交互放进组件：去掉组件，回复仍然回答了问题。该用而没用，和不该用而用了，同样是错。</principle>`

func liveUseRule(panel, widget bool) string {
	signals := []string{"同类条目超过五行（清单、候选、商品、城市、条目），或者需要筛选、排序、搜索", "用户在估算、比较方案或做 what-if，哪怕参数由你假设"}
	if panel {
		signals = append(signals, "讲解公式、定律或有数量关系的概念（物理、化学、金融、统计、几何），用 panel 放公式、滑块和示意图，让用户拖动参数看结果怎么变，正文照常讲解")
	}
	if widget {
		signals = append(signals, "讲解随时间变化的过程或空间结构（动画、三维示意、流场、机构运动），用 <widget>")
	}
	signals = append(signals, "给到或查到一组数，有趋势、分布、构成或多组对比", "过程要一步步走（排障、选型、资格判断、自测）", "有日期、时长或先后关系", "修改前后的对照")
	return "  <use>满足任一项就出组件：" + strings.Join(signals, "；") + "。拿不准时看用户接下来会不会想改一个数、筛一遍或点一下；会，就出组件。引用工具结果时按原样放进组件，不要改写成文字。</use>"
}

const livePromptAvoid = `  <avoid>这些条件优先于 <use>，命中就退回文字或 Markdown：闲聊、观点、没有可调的量的定性概念、写作类请求（邮件、文案、代码）、一两句话就能答完；数据很少（三个以内且没有趋势的数字），直接写在正文或 Markdown 表格里；用户要可复制的纯文本、指定格式，或说过不要组件；没有可靠数据，不要为了画图编造数字，估算须在正文写明依据；组件只是把正文已有的内容再摆一遍；用组件代替回答，比如先发一张表单让用户选题材、篇幅、风格再动手，能直接回答就按合理的默认直接答，需要确认时用一句话问。</avoid>`

func liveChooseRule(panel, widget bool) string {
	tiers := []string{"目录组件"}
	var rules []string
	if panel {
		tiers = append(tiers, "panel")
		rules = append(rules, "只有存在会改变结果的控件，或几个部件要共享状态时才用 panel，不要用 panel 包一个静态组件")
	}
	if widget {
		tiers = append(tiers, "widget")
		rules = append(rules, "只有需要动画、三维或大量图形时才用 widget")
	}
	choose := "  <choose>一条回复一般一个块，最多三个，一个块对应一个需求。"
	if len(rules) > 0 {
		choose += "由轻到重：" + strings.Join(tiers, " → ") + "，能用轻的就不用重的。" + strings.Join(rules, "；") + "。"
	}
	return choose + "</choose>"
}

const livePromptRules = `  <followup>用户在组件里的改动会在下一轮以 ui_state 提供。追问已有组件时，直接用其中的当前值作答，不要让用户复述，也不重复输出同一个块；用户要求改组件时，输出一个新块（新 id），初始值取用户当前的值。</followup>
  <check>写完回看一遍：有明确的 <use> 信号却没有组件（用户明确要纯文本时除外），或者没有信号却放了组件，都算错，按本节重写。</check>
</when>
<syntax>语言标识为 ` + "`" + domainuicomponent.FenceLanguage + "`" + ` 的代码块，内容是 JSON：component、id（简短英文，只用字母、数字、- 和 _，对话内唯一）与组件属性；属性可以平铺在顶层，也可以放进 props。字符串内的英文双引号写成 \"，不要尾随逗号或注释。代码块放在正文中它所属的位置，前后用自然语言承接，不要在组件外重复组件里的数据。
` + "```" + domainuicomponent.FenceLanguage + `
{"component": "stat-grid", "id": "kpi-week", "title": "本周指标", "items": [{"label": "日活", "value": "128,430", "delta": "+4.2%", "trend": "up"}]}
` + "```" + `</syntax>`

// livePromptPanel 只在勾选了 panel 时出现。
var livePromptPanel = `<live>
  <state>state 是用户可改的初始值；computed 是由 state 推导的值，写不带花括号的公式，可以互相引用，不能循环。</state>
  <children>children 从上到下排列，可以是下面的原语，也可以是 catalog 里的组件（嵌套时不写 id）。控件用 bind 指向 state 名；visible: "{条件}" 为假时隐藏该节点。</children>
  <formula>属性值里的 {公式} 实时求值：整个值只有一个 {…} 时保留原类型（数字、列表），与文字混写时数字按千分位、最多两位小数显示；{{ 和 }} 表示字面花括号。公式只能用数字、'文字'、+ - * / % ^、比较、&& || !、a ? b : c、obj.key、list[i]、[a, b]，函数 abs sqrt cbrt floor ceil sign log ln log10 log2 exp sin cos tan asin acos atan sinh cosh tanh pow min max round(x, 位数?) fixed(x, 位数) clamp(x, 下限, 上限) range(起, 止, 步长?) sum avg len contains，常量 pi e；没有赋值、循环和自定义函数。range 包含两端，最多 1000 项。</formula>
  <data>chart 的 series[].data 可以写成关于 x 的公式，按 x 列表逐点计算（i 是下标）；data-table 与 card-grid 可以加 where: "{关于 row 的条件}" 筛选行。</data>
  <actions>button 三选一：ask: "文字" 显示为一个可编辑的追问框，预填这段文字（可以含 {公式}），用户可以改写后点发送，作为下一条消息发出，所以写成用户可能想问的完整问题；set: {state 名: 值} 修改状态；copy: "文字" 复制到剪贴板。</actions>
  <primitives>
    布局：panel {title?, description?}、row、stack、grid {columns?: 1-4}、card {title?}、tabs {items: [标签], bind?}（第 i 个子节点是第 i 个标签页）、collapse {title, open?}、divider
    展示：text {text, tone?, size?: sm|base|lg}、stat {label, value, unit?, note?, precision?, highlight?, tone?}、badge {text, tone?}、progress {value, max?, label?}、callout {text, title?, tone?: info|positive|warning|negative}
    讲解：formula {expr, name?, unit?, precision?, label?, with?}（expr 是不带花括号的公式，按当前值代入，显示「公式 = 代入 = 结果」；写成课本里的形式如 G * M / r^2，常数放进 computed，不要把算好的数折进公式；滑块用的单位与公式不同时，用 with: {"M": "{M * 1e24}"} 给出代入公式的值）、scene {width?, height?, shapes: [{type: rect|circle|line|arrow|text, 坐标, label?, tone?}]}（示意图，按实际像素画，宽 240–400 为宜，原点在左上；rect {x, y, w, h}、circle {cx, cy, r}、line/arrow {x1, y1, x2, y2}、text {x, y, text}，坐标与 label 都可以写 {公式}；只画说明关系必需的几个图形，离边缘与彼此留出 16 以上）
    版式：panel 的 children 平铺写即可。有 scene 时，客户端自动把公式与控件放左边、示意图放右边、ask 按钮放下方，不用自己写 row
    控件：slider {bind, label, min, max, step?, unit?}、select {bind, label, options}、segmented {bind, label?, options}、switch {bind, label}、input {bind, label, type?: text|number, placeholder?}、button {label?, ask | set | copy, variant?: default|outline}
    tone 取 default|muted|positive|negative|warning|info；options 写成 ["文字"] 或 [{label, value}]
  </primitives>
  <example>
` + "```" + domainuicomponent.FenceLanguage + "\n" + strings.TrimSpace(liveExample) + "\n```" + `
  </example>
  <example>
` + "```" + domainuicomponent.FenceLanguage + "\n" + strings.TrimSpace(explorableExample) + "\n```" + `
  </example>
</live>`

// livePromptWidget 只在勾选了 widget 时出现。
const livePromptWidget = `<widget>
  <use>讲解需要动画、时间推进、三维或透视示意、粒子与流场、几何作图、机构或工程过程演示，而 panel 与 scene 表达不了时，在正文里写一个内联交互图解。用户要完整网页、小游戏或可下载的代码时仍用 html Artifact。</use>
  <format>语言标识为 ` + "`" + domainuicomponent.WidgetFenceLanguage + "`" + ` 的代码块，内容是自包含的 HTML 片段，不写 html、head、body 标签：<title> 写图解标题，<style> 与 <script> 内联，用 SVG 或 canvas 绘制，图例、播放控件、说明文字都放在图解里。它在沙箱里运行且没有网络：不要引用外部脚本、样式、字体、图片或 CDN，不要用 alert、prompt、表单提交、localStorage、cookie。代码块写完后才会运行。</format>
  <design>宽度占满容器，高度不超过 640px；颜色只用主题变量 var(--foreground)、var(--muted-foreground)、var(--primary)、var(--border)、var(--card)、var(--chart-1) 到 var(--chart-5)，深色与浅色都要可读，背景留透明；标签不要压在图形上；有动画时提供播放与暂停，并尊重 prefers-reduced-motion；代码控制在 12KB 以内，写得完整，不要省略号或待补充。</design>
  <layout>宿主已留出 16px 内边距，并给 button、select、input 设好了与界面一致的样式，外层不要再加边距，控件不要另写样式。从上到下：标题与一句说明、图形、随图形变化的说明文字、控件行。控件放进一个 class="controls" 的容器排成一行（播放/暂停、速度、进度），按下状态用 aria-pressed="true"。随状态变化的文字不要直接改写：把每种情况的文案都写成 class="stable" 容器的子元素，当前那段加 class="on"，切换时只移动 on，容器高度自动取最长那段；变化的数字加 class="num" 并给最小宽度。控件不能因为文字长短或行数变化而移动。追问不要自己写输入框或按钮：调用 deeix.suggest("问题") 给出一个用户可能想问的完整问题（120 字以内，可以带上当前的参数），宿主会在图解下方显示与界面一致的追问框，用户可以改写后发送；状态变化时可以再次调用来更新。</layout>
  <api>window.deeix.getState()、onState(fn)、setState({名: 值}) 保存用户的选择（名用字母、数字、- 和 _，值是 JSON，合计 4KB 内）：刷新后恢复，下一轮以 ui_state 提供给你。deeix.suggest("文字") 设置图解下方的追问，传空字符串则不显示。deeix.onTheme(fn) 在用户切换深浅色时回调。</api>
</widget>`

// CatalogPrompt 生成注入系统提示词的组件目录层。blockFormat 是客户端声明的块格式：静态格式（及未声明）
// 的输出必须逐字稳定，不含 panel 与 widget；可交互格式按勾选的能力写入 <live> 与 <widget>。
func CatalogPrompt(components []domainuicomponent.Component, blockFormat int) string {
	live := blockFormat >= domainuicomponent.BlockFormatLive
	catalog := withoutCapabilities(components)
	panel, widget := false, false
	if live {
		panel, widget = Capabilities(components)
	}
	if panel {
		catalog = withoutSuperseded(catalog)
	}
	if len(catalog) > maxCatalogComponents {
		catalog = catalog[:maxCatalogComponents]
	}
	if len(catalog) == 0 && !panel && !widget {
		return ""
	}
	var builder strings.Builder
	if live {
		builder.WriteString(liveCatalogHeader(panel, widget))
		if len(catalog) == 0 {
			return builder.String()
		}
		builder.WriteString("\n<catalog>")
	} else {
		builder.WriteString(catalogPromptHeader)
	}
	for _, component := range catalog {
		builder.WriteString("\n  <component name=\"")
		builder.WriteString(component.Name)
		builder.WriteString("\"")
		if component.Version > 1 {
			builder.WriteString(" version=\"")
			builder.WriteString(strconv.Itoa(component.Version))
			builder.WriteString("\"")
		}
		builder.WriteString(">\n    <use>")
		use, _, _ := strings.Cut(component.Description, descriptionInteractions)
		builder.WriteString(use)
		builder.WriteString("</use>\n    <props>")
		props, rules, hasRules := strings.Cut(component.PropsSummary, propsRulesSeparator)
		builder.WriteString(props)
		builder.WriteString("</props>")
		if hasRules {
			builder.WriteString("\n    <rules>")
			builder.WriteString(rules)
			builder.WriteString("</rules>")
		}
		builder.WriteString("\n  </component>")
	}
	builder.WriteString(catalogPromptFooter)
	return builder.String()
}

// Capabilities 报告本次勾选了哪些块格式能力；只认内置行，自定义组件不能用这两个名字。
func Capabilities(components []domainuicomponent.Component) (panel, widget bool) {
	for _, component := range components {
		if component.RendererKind != domainuicomponent.RendererBuiltin {
			continue
		}
		switch component.Name {
		case domainuicomponent.CapabilityPanel:
			panel = true
		case domainuicomponent.CapabilityWidget:
			widget = true
		}
	}
	return panel, widget
}

// withoutCapabilities 去掉能力行：它们不是可以写进块里的组件，不进入 <catalog>。
func withoutCapabilities(components []domainuicomponent.Component) []domainuicomponent.Component {
	kept := make([]domainuicomponent.Component, 0, len(components))
	for _, component := range components {
		if component.RendererKind == domainuicomponent.RendererBuiltin && domainuicomponent.IsCapability(component.Name) {
			continue
		}
		kept = append(kept, component)
	}
	return kept
}

// withoutSuperseded 去掉被 panel 取代的内置组件；按渲染方式判断，同名的自定义（沙箱）组件不受影响。
func withoutSuperseded(components []domainuicomponent.Component) []domainuicomponent.Component {
	kept := make([]domainuicomponent.Component, 0, len(components))
	for _, component := range components {
		if component.RendererKind == domainuicomponent.RendererBuiltin && slices.Contains(domainuicomponent.SupersededByLive, component.Name) {
			continue
		}
		kept = append(kept, component)
	}
	return kept
}

// CreateUser 创建用户自定义组件。
func (s *Service) CreateUser(ctx context.Context, userID uint, input WriteInput) (*domainuicomponent.Component, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainuicomponent.ScopeUser, userID, userID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// CreatePlatform 创建管理员平台组件。
func (s *Service) CreatePlatform(ctx context.Context, actorUserID uint, input WriteInput) (*domainuicomponent.Component, error) {
	if actorUserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := normalizeWriteInput(input, domainuicomponent.ScopePlatform, 0, actorUserID)
	if err != nil {
		return nil, err
	}
	return s.create(ctx, item)
}

// UpdateUser 更新当前用户自定义组件。
func (s *Service) UpdateUser(ctx context.Context, userID uint, id uint, input PatchInput) (*domainuicomponent.Component, error) {
	if userID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	if item.Scope != domainuicomponent.ScopeUser || item.OwnerUserID != userID {
		return nil, ErrComponentNotFound
	}
	return s.update(ctx, item, userID, input)
}

// UpdateAdmin 更新内置或平台组件。内置组件只允许修改启用状态、描述与排序。
func (s *Service) UpdateAdmin(ctx context.Context, actorUserID uint, id uint, input PatchInput) (*domainuicomponent.Component, error) {
	if actorUserID == 0 || id == 0 {
		return nil, repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	switch item.Scope {
	case domainuicomponent.ScopePlatform:
		return s.update(ctx, item, actorUserID, input)
	case domainuicomponent.ScopeBuiltin:
		// 内置组件的目录内容由代码定义并在启动时同步，管理员只能启停。
		if input.Name != nil || input.Version != nil || input.Description != nil || input.PropsSummary != nil || input.PropsSchema != nil || input.RendererSource != nil {
			return nil, ErrBuiltinProtected
		}
		return s.update(ctx, item, actorUserID, input)
	default:
		return nil, ErrComponentNotFound
	}
}

// DeleteUser 删除当前用户自定义组件。
func (s *Service) DeleteUser(ctx context.Context, userID uint, id uint) error {
	if userID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	if item.Scope != domainuicomponent.ScopeUser || item.OwnerUserID != userID {
		return ErrComponentNotFound
	}
	return mapRepositoryError(s.repo.DeleteUIComponent(ctx, id))
}

// DeleteAdmin 删除平台组件。内置组件受保护。
func (s *Service) DeleteAdmin(ctx context.Context, actorUserID uint, id uint) error {
	if actorUserID == 0 || id == 0 {
		return repository.ErrInvalidInput
	}
	item, err := s.repo.GetUIComponent(ctx, id)
	if err != nil {
		return mapRepositoryError(err)
	}
	switch item.Scope {
	case domainuicomponent.ScopeBuiltin:
		return ErrBuiltinProtected
	case domainuicomponent.ScopePlatform:
		return mapRepositoryError(s.repo.DeleteUIComponent(ctx, id))
	default:
		return ErrComponentNotFound
	}
}

func (s *Service) create(ctx context.Context, item *domainuicomponent.Component) (*domainuicomponent.Component, error) {
	result, err := s.repo.CreateUIComponent(ctx, item)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return result, nil
}

func (s *Service) update(ctx context.Context, current *domainuicomponent.Component, actorUserID uint, input PatchInput) (*domainuicomponent.Component, error) {
	patch, err := normalizePatchInput(input, current, actorUserID)
	if err != nil {
		return nil, err
	}
	item, err := s.repo.PatchUIComponent(ctx, current.ID, patch)
	if err != nil {
		return nil, mapRepositoryError(err)
	}
	return item, nil
}

func normalizeWriteInput(input WriteInput, scope string, ownerUserID uint, actorUserID uint) (*domainuicomponent.Component, error) {
	name := strings.TrimSpace(input.Name)
	if !validName(name) {
		return nil, ErrInvalidComponent
	}
	version := input.Version
	if version <= 0 {
		version = 1
	}
	description := strings.TrimSpace(input.Description)
	summary := strings.TrimSpace(input.PropsSummary)
	schema := strings.TrimSpace(input.PropsSchema)
	source := input.RendererSource
	if description == "" || runeCount(description) > maxDescriptionLength || summary == "" || runeCount(summary) > maxPropsSummaryLength {
		return nil, ErrInvalidComponent
	}
	if err := validSchema(schema); err != nil {
		return nil, err
	}
	if strings.TrimSpace(source) == "" || len(source) > maxRendererSourceBytes {
		return nil, ErrInvalidComponent
	}
	return &domainuicomponent.Component{
		Scope:           scope,
		OwnerUserID:     ownerUserID,
		Name:            name,
		Version:         version,
		Description:     description,
		PropsSummary:    summary,
		PropsSchema:     schema,
		RendererKind:    domainuicomponent.RendererSandbox,
		RendererSource:  source,
		Enabled:         input.Enabled,
		SortOrder:       input.SortOrder,
		CreatedByUserID: actorUserID,
		UpdatedByUserID: actorUserID,
	}, nil
}

func normalizePatchInput(input PatchInput, current *domainuicomponent.Component, actorUserID uint) (repository.UIComponentPatch, error) {
	patch := repository.UIComponentPatch{UpdatedByUserIDSet: true, UpdatedByUserID: actorUserID}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if !validName(name) {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Name = &name
	}
	if input.Version != nil {
		if *input.Version <= 0 {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Version = input.Version
	}
	if input.Description != nil {
		description := strings.TrimSpace(*input.Description)
		if description == "" || runeCount(description) > maxDescriptionLength {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.Description = &description
	}
	if input.PropsSummary != nil {
		summary := strings.TrimSpace(*input.PropsSummary)
		if summary == "" || runeCount(summary) > maxPropsSummaryLength {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.PropsSummary = &summary
	}
	if input.PropsSchema != nil {
		schema := strings.TrimSpace(*input.PropsSchema)
		if err := validSchema(schema); err != nil {
			return repository.UIComponentPatch{}, err
		}
		patch.PropsSchema = &schema
	}
	if input.RendererSource != nil {
		if current.RendererKind != domainuicomponent.RendererSandbox {
			return repository.UIComponentPatch{}, ErrBuiltinProtected
		}
		if strings.TrimSpace(*input.RendererSource) == "" || len(*input.RendererSource) > maxRendererSourceBytes {
			return repository.UIComponentPatch{}, ErrInvalidComponent
		}
		patch.RendererSource = input.RendererSource
	}
	patch.Enabled = input.Enabled
	patch.SortOrder = input.SortOrder
	return patch, nil
}

// validName 校验组件名。原语名会与 panel 里的节点混淆，能力名（panel、widget）会与内置目录行混淆，都不允许。
func validName(name string) bool {
	return name != "" && runeCount(name) <= maxNameLength && componentNamePattern.MatchString(name) && !domainuicomponent.IsPrimitive(name) && !domainuicomponent.IsCapability(name)
}

// validSchema 只要求是合法 JSON 对象且不超长；结构校验在前端按 JSON Schema 执行。
func validSchema(schema string) error {
	if schema == "" {
		return nil
	}
	if len(schema) > maxPropsSchemaBytes {
		return ErrInvalidComponent
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(schema), &payload); err != nil {
		return ErrInvalidComponent
	}
	return nil
}

func uniqueIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func runeCount(value string) int {
	return len([]rune(value))
}

func mapRepositoryError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, repository.ErrNotFound) {
		return ErrComponentNotFound
	}
	if errors.Is(err, repository.ErrDuplicate) {
		return ErrComponentConflict
	}
	if errors.Is(err, repository.ErrInvalidInput) {
		return ErrInvalidComponent
	}
	return err
}
