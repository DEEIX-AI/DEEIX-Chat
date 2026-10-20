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

const livePromptIntro = `<nature>交互式组件是嵌入回复正文的 Markdown 代码块，由客户端渲染为界面。它是一种输出格式，而非工具：不得通过 function calling / tool call 调用，也无需等待返回结果。</nature>
<when>
  <principle>组件用于让用户直接操作与读取数据，而非装饰。命中 <use> 的任一条件时，默认应输出组件，仅以文字转述数据视为未完成；均未命中时，采用最轻量的表达：纯文字，其次为 Markdown 列表或表格。结论与要点写在正文，明细与交互放入组件；移除组件后，正文仍须完整回答问题。应使用而未使用，与不应使用而使用，同属错误。</principle>`

func liveUseRule(panel, widget bool) string {
	signals := []string{"同类条目超过五行（清单、候选、商品、城市），或需要筛选、排序、检索", "估算、方案比较或假设分析（what-if），即使参数由你设定"}
	if panel {
		signals = append(signals, "讲解公式、定律或含数量关系的概念（物理、化学、金融、统计、几何）：用 panel 呈现公式、滑块与示意图，供用户调整参数并观察结果变化，正文照常讲解")
	}
	if widget {
		signals = append(signals, "讲解随时间演变的过程或空间结构（动画、三维或透视示意、粒子与流场、几何作图、机构或工程过程）：用 <widget>")
	}
	signals = append(signals, "提供或检索到一组数据，且涉及趋势、分布、构成或多组对比", "需要分步推进的流程（排障、选型、资格判定、自测）", "多个任务或事件具有起止日期、时长或依赖关系（排期、里程碑）", "用户需要查看修改前后的差异")
	return "  <use>满足任一条件即输出组件：" + strings.Join(signals, "；") + "。难以判断时，考虑用户下一步是否会调整数值、筛选或点选；若会，则输出组件。工具结果符合上述条件时，原样放入组件，不得改写为文字。</use>"
}

const livePromptAvoid = `  <avoid>以下情形优先于 <use>，命中任一即改用文字或 Markdown：闲聊、观点表达、不含可调量的定性概念、一两句话即可答完的问题；写作类请求（邮件、文案、代码）直接给出成品，仅当用户需要查看改动时才使用对照；数据量很少（不超过三个且无趋势的数值），直接写入正文或 Markdown 表格；用户要求可复制的纯文本或指定格式，或明确表示不需要组件；缺乏可靠数据时，不得为作图编造数字，估算须在正文注明依据；组件仅重复正文已有内容；以组件代替回答，例如先发送表单让用户选择题材、篇幅、风格再作答。能直接回答时按合理的默认值作答，确需确认时以一句话询问。</avoid>`

func liveChooseRule(panel, widget bool) string {
	tiers := []string{"目录组件"}
	var rules []string
	if panel {
		tiers = append(tiers, "panel")
		rules = append(rules, "仅当存在影响结果的控件，或多个部件需要共享状态时使用 panel，不得用 panel 包裹静态组件")
	}
	if widget {
		tiers = append(tiers, "widget")
		rules = append(rules, "仅当需要动画、三维或大量图形时使用 widget")
	}
	choose := "  <choose>每条回复通常包含一个块，至多三个，每个块对应一项需求。"
	if len(rules) > 0 {
		choose += "按由轻到重的顺序选择：" + strings.Join(tiers, " → ") + "，轻量方案能够满足时不采用更重的方案。" + strings.Join(rules, "；") + "。"
	}
	return choose + "</choose>"
}

const livePromptRules = `  <followup>用户在组件中的操作会在下一轮以 ui_state 提供。针对已有组件的追问，直接使用其中的当前值作答，不要求用户复述，也不重复输出同一个块；用户要求修改组件时，输出一个新块（使用新 id），初始值沿用用户的当前值。</followup>
</when>
<syntax>语言标识为 ` + "`" + domainuicomponent.FenceLanguage + "`" + ` 的代码块，内容为 JSON：component、id（简短英文，仅含字母、数字、- 和 _，对话内唯一），组件属性平铺于顶层。字符串中的英文双引号须转义为 \"，不得包含尾随逗号或注释。代码块置于正文中对应的位置，前后以自然语言衔接；正文给出结论与要点，不逐条复述组件中的明细。
` + "```" + domainuicomponent.FenceLanguage + `
{"component": "stat-grid", "id": "kpi-week", "title": "本周指标", "items": [{"label": "日活", "value": "128,430", "delta": "+4.2%", "trend": "up"}]}
` + "```" + `</syntax>`

// livePromptPanel 只在勾选了 panel 时出现。
var livePromptPanel = `<live>
  <state>state 为用户可修改的初始值；computed 为由 state 推导的值，以不带花括号的公式书写，可相互引用，但不得循环引用。</state>
  <children>children 自上而下排列，可以是下列原语，也可以是 catalog 中的组件（嵌套时省略 id）。控件通过 bind 绑定 state 名；visible: "{条件}" 为假时隐藏该节点。</children>
  <formula>属性值中的 {公式} 实时求值：整个值仅含一个 {…} 时保留原类型（数字、列表）；与文字混排时，数字按千分位、至多两位小数显示；{{ 与 }} 表示字面花括号。公式仅支持数字、'文字'、+ - * / % ^、比较运算、&& || !、a ? b : c、obj.key、list[i]、[a, b]；函数 abs sqrt cbrt floor ceil sign log ln log10 log2 exp sin cos tan asin acos atan sinh cosh tanh pow min max round(x, 位数?) fixed(x, 位数) clamp(x, 下限, 上限) range(起, 止, 步长?) sum avg len contains；常量 pi e。不支持赋值、循环与自定义函数。range 包含两端，至多 1000 项。</formula>
  <data>chart 的 series[].data 可写为关于 x 的公式，按 x 列表逐点计算（i 为下标）；data-table 与 card-grid 可通过 where: "{关于 row 的条件}" 筛选行。</data>
  <actions>button 取以下三种动作之一：ask: "文字" 显示为可编辑的追问框并预填该文字（可含 {公式}），用户修改后发送，作为下一条消息，因此应写成用户可能提出的完整问题；set: {state 名: 值} 修改状态；copy: "文字" 复制到剪贴板。</actions>
  <primitives>
    布局：panel {title?, description?}、row、stack、grid {columns?: 1-4}、card {title?}、tabs {items: [标签], bind?}（第 i 个子节点对应第 i 个标签页）、collapse {title, open?}、divider
    展示：text {text, tone?, size?: sm|base|lg}、stat {label, value, unit?, note?, precision?, highlight?, tone?}、badge {text, tone?}、progress {value, max?, label?}、callout {text, title?, tone?: info|positive|warning|negative}
    讲解：formula {expr, name?, unit?, precision?, label?, with?}（expr 为不带花括号的公式，按当前值代入，显示为「公式 = 代入 = 结果」；按教材写法书写，如 G * M / r^2，常数放入 computed，不得将计算结果直接写入公式；滑块单位与公式单位不一致时，用 with: {"M": "{M * 1e24}"} 指定代入公式的值）、scene {width?, height?, shapes: [{type: rect|circle|line|arrow|text, 坐标, label?, tone?}]}（示意图，按实际像素绘制，宽度以 240–400 为宜，原点位于左上角；rect {x, y, w, h}、circle {cx, cy, r}、line/arrow {x1, y1, x2, y2}、text {x, y, text}，坐标与 label 均可使用 {公式}；仅绘制说明关系所必需的图形，与边缘及彼此的间距不小于 16）
    版式：panel 的 children 平铺书写即可。含 scene 时，客户端自动将公式与控件置于左侧、示意图置于右侧、ask 按钮置于下方，无需手写 row
    控件：slider {bind, label, min, max, step?, unit?}、select {bind, label, options}、segmented {bind, label?, options}、switch {bind, label}、input {bind, label, type?: text|number, placeholder?}、button {label?, ask | set | copy, variant?: default|outline}
    tone 取值 default|muted|positive|negative|warning|info；options 写作 ["文字"] 或 [{label, value}]
  </primitives>
  <example>
` + "```" + domainuicomponent.FenceLanguage + "\n" + strings.TrimSpace(liveExample) + "\n```" + `
  </example>
  <example>
` + "```" + domainuicomponent.FenceLanguage + "\n" + strings.TrimSpace(explorableExample) + "\n```" + `
  </example>
</live>`

// livePromptWidget 只在勾选了 widget 时出现。<layout> 里的类名与 --positive 等变量由宿主注入的基础样式提供
// （apps/web/shared/components/markdown/ui-blocks/sandbox-host.tsx 的 WIDGET_BASE_STYLE），两边须同步修改。
const livePromptWidget = `<widget>
  <use>嵌入正文的交互式图解。用户需要完整网页、小游戏或可下载的代码时，使用 html Artifact，而非 widget。</use>
  <format>语言标识为 ` + "`" + domainuicomponent.WidgetFenceLanguage + "`" + ` 的代码块，内容为自包含的 HTML 片段，不含 html、head、body 标签：<title> 是提供给宿主的名称，不会显示；<style> 与 <script> 内联；使用 SVG 或 canvas 绘制，图例、播放控件与说明文字均置于图解内。图解运行于没有网络的沙箱：不得引用外部脚本、样式、字体、图片或 CDN，不得使用 alert、prompt、表单提交、localStorage、cookie。</format>
  <design>宽度占满容器，高度宜控制在 640px 以内，超出时以分组或折叠收纳；宿主按内容自动调整高度，不得设置 max-height 或自建滚动区域。颜色仅使用主题变量 var(--foreground)、var(--muted-foreground)、var(--primary)、var(--border)、var(--card)、var(--chart-1) 至 var(--chart-5)，好坏状态使用 var(--positive)、var(--negative)、var(--warning)，须在深浅两种主题下均清晰可读。用色克制：--primary 仅用于选中状态与最关键的一项结果；chart 色仅用于图形中的数据系列，不用于表示好坏或文字着色。文字不使用全大写或额外字距；标签不得遮挡图形。含动画时须提供播放与暂停，并遵循 prefers-reduced-motion。代码控制在 12KB 以内，须完整书写，不得使用省略号或占位内容。</design>
  <layout>宿主外框即为卡片，已预留 16px 内边距，并为 button、select、input、table 提供与界面一致的样式：外层不再添加边距、边框、背景或阴影，背景保持透明；控件与表格不再另设颜色、边框与字号。分组依靠留白、h4 小标题与分隔线；需要成块时使用 class="card"，可选中的卡片写作 button.card，选中时加 aria-pressed="true"，卡片之间不要互相嵌套。宿主另提供以下类：row（横向排列）、stack（纵向排列并保留组间距）、grid（自适应等宽网格）、label（小号说明）、value（大号数值）、badge（可叠加 positive、negative、warning）、chip（多选按钮，选中时使用 aria-pressed）；直接使用这些类，仅为布局编写样式。图解自上而下依次为：可见的标题与一句说明、图形、随图形变化的说明文字、控件行。控件置于 class="controls" 的容器中单行排列（播放/暂停、速度、进度），按下状态使用 aria-pressed="true"。随状态变化的文字不得直接改写：将各状态的文案作为 class="stable" 容器的子元素，当前项加 class="on"，切换时仅移动 on，容器高度自动取最长项；变化的数字加 class="num"（等宽数字，无需另设等宽字体）并设定最小宽度。控件位置不得随文字长度或行数变化而移动。追问不得自行绘制输入框或按钮，统一使用 deeix.suggest。</layout>
  <api>window.deeix.getState()、onState(fn)、setState({名: 值}) 用于保存用户的选择（名称仅含字母、数字、- 和 _，值为 JSON，总计不超过 4KB）：刷新后恢复，并在下一轮以 ui_state 提供给你。deeix.suggest("问题") 在图解下方显示与界面一致的追问框，预填一个用户可能提出的完整问题（不超过 120 字，可包含当前参数），用户可修改后发送；状态变化时可再次调用以更新，传入空字符串则隐藏。deeix.onTheme(fn) 在用户切换深浅色主题时回调。</api>
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
