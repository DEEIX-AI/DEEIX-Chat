// Package uicomponent 定义对话中可渲染的交互式组件。
package uicomponent

import "time"

// FenceLanguage 是模型输出组件时使用的 Markdown 围栏语言标识。
const FenceLanguage = "deeix-ui"

const (
	// ScopeBuiltin 表示仓库内置组件；启动时播种，受保护不可删除。
	ScopeBuiltin = "builtin"
	// ScopePlatform 表示管理员创建的全局组件。
	ScopePlatform = "platform"
	// ScopeUser 表示用户创建的个人组件。
	ScopeUser = "user"
)

const (
	// RendererBuiltin 表示前端注册表按 Name 与 Version 分发的 React 渲染器。
	RendererBuiltin = "builtin"
	// RendererSandbox 表示 RendererSource 中的 HTML 在沙箱 iframe 内渲染。
	RendererSandbox = "sandbox"
)

// Component 表示一个可供模型选用、由前端渲染的组件。
type Component struct {
	ID          uint
	Scope       string
	OwnerUserID uint
	Name        string
	Version     int
	Description string
	// PropsSummary 是给模型看的紧凑入参说明，例如 `{title, items[{tag,title}]}`。
	PropsSummary string
	// PropsSchema 是 JSON Schema 文本，前端据此校验自定义组件的 props。内置组件为空。
	PropsSchema     string
	RendererKind    string
	RendererSource  string
	Enabled         bool
	SortOrder       int
	CreatedByUserID uint
	UpdatedByUserID uint
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// builtinSpec 用统一的结构描述内置组件：Description 由 What / When / Interactions 生成（≤ 256 字，
// 也是选择器里展示的文案）；入参约定 Rules 跟 Props 一起进入 PropsSummary，只给模型看。
type builtinSpec struct {
	Name    string
	Version int
	// What 一句话定位，也是选择器里显示的首句。
	What string
	// When 适用场景。
	When string
	// Interactions 用户可做的交互，说明它比纯 Markdown 多出的价值。
	Interactions string
	// Rules 入参约定，可空；只出现在给模型的入参摘要里。
	Rules string
	Props string
}

func (spec builtinSpec) description() string {
	return spec.What + "。适用：" + spec.When + "。交互：" + spec.Interactions
}

func (spec builtinSpec) propsSummary() string {
	if spec.Rules == "" {
		return spec.Props
	}
	return spec.Props + "。约定：" + spec.Rules
}

var builtinSpecs = []builtinSpec{
	{
		Name:         "card-grid",
		Version:      1,
		What:         "卡片网格",
		When:         "新闻、搜索结果、候选方案等同构条目",
		Interactions: "按 tag 筛选（条目带 tag 时自动出现）；带 url 的卡片可点击",
		Props:        "{title, items: [{title, summary?, tag?, source?, url?}]}",
	},
	{
		Name:         "chart",
		Version:      1,
		What:         "数据图表",
		When:         "折线、柱状、面积、横向柱状、散点、饼图、雷达、漏斗、进度环",
		Interactions: "图例切换序列显隐；悬停读数",
		Rules:        "scatter 的 x 为数值；series.type 可单独改画柱/线/面积，series.axis: right 用第二 Y 轴；percent 需配合 stacked",
		Props:        "{title?, type: line|bar|area|horizontal-bar|scatter|pie|radar|funnel|radial, x?: (string|number)[], series: [{name, data: number[], type?: line|bar|area, axis?: left|right}], unit?, stacked?: boolean, percent?: boolean}",
	},
	{
		Name:         "function-plot",
		Version:      1,
		What:         "函数图像与方程曲线",
		When:         "讲解数学函数、方程、几何曲线",
		Interactions: "编辑、增删表达式实时重绘；滚轮缩放、拖动平移；显隐单条曲线",
		Rules:        "expr 可写 x 的表达式、y = …、x = …（关于 y）或含 x、y 的方程；乘法必须写 *；支持常见数学函数（sin cos tan sqrt abs exp ln log pow floor ceil round min max 等）与 pi、e",
		Props:        "{title?, functions: [{expr}], x?: [min, max], y?: [min, max]}",
	},
	{
		Name:         "stat-grid",
		Version:      1,
		What:         "关键指标卡片组",
		When:         "几个并列的数字：KPI、涨跌幅、统计值",
		Interactions: "带 history 的指标显示迷你走势线，悬停逐点读数",
		Rules:        "trend 决定颜色；history 从旧到新，periods 是各点标签",
		Props:        "{title?, periods?: string[], items: [{label, value, unit?, delta?, trend?: up|down|flat, note?, history?: number[]}]}",
	},
	{
		Name:         "gantt",
		Version:      1,
		What:         "甘特图",
		When:         "项目排期、里程碑、任务依赖",
		Interactions: "日/周/月刻度切换；点击任务高亮整条依赖链；分组折叠；标出今天与周末",
		Rules:        "日期 YYYY-MM-DD；end 与 days 二选一；depends 引用其它任务的 id（缺省为 name）",
		Props:        "{title?, tasks: [{id?, name, start, end?, days?: number, group?, progress?: 0-100, milestone?: boolean, depends?: string[]}]}",
	},
	{
		Name:         "data-table",
		Version:      1,
		What:         "数据表格",
		When:         "十行以上的结构化数据",
		Interactions: "排序、搜索、分页、导出 CSV",
		Rules:        "column.type 决定排序方式与对齐",
		Props:        "{title?, columns: [{key, label, type?: string|number|date|boolean}], rows: [{<key>: value}], pageSize?: number}",
	},
	{
		Name:         "calculator",
		Version:      1,
		What:         "交互式计算器",
		When:         "贷款、利率、单位换算、预算等 what-if 分析",
		Interactions: "拖动输入滑块，输出按公式实时重算",
		Rules:        "expr 只能用四则运算、^、括号、输入 key 与常见数学函数；highlight 标出主结果",
		Props:        "{title?, description?, inputs: [{key, label, min, max, step?, default, unit?}], outputs: [{label, expr, unit?, precision?, highlight?: boolean}]}",
	},
	{
		Name:         "decision-tree",
		Version:      1,
		What:         "引导式决策树",
		When:         "排障、选型、资格判断",
		Interactions: "逐步点选进入下一节点，可返回或重置",
		Rules:        "没有 options 的节点是结论；start 缺省为第一个节点",
		Props:        "{title?, start?, nodes: [{id, text, detail?, options?: [{label, next}]}]}",
	},
	{
		Name:         "diff",
		Version:      1,
		What:         "文本 / 代码对比",
		When:         "修改前后、版本差异、重构对照",
		Interactions: "合并 / 并排视图切换；逐处跳转修改；折叠未变化行；复制修改后内容；行内改动按词高亮",
		Props:        "{title?, before, after, language?, beforeLabel?, afterLabel?}",
	},
	{
		Name:         "quiz",
		Version:      1,
		What:         "选择题测验 / 分支问答",
		When:         "学习检验、知识自测、向导式问答",
		Interactions: "点选即时判对错并显示解析，统计得分；带 next 时逐题呈现、按选择跳题、可返回",
		Rules:        "answer 是正确选项下标，省略则只分流不计分；选项写成 {text, next} 可跳到指定 id 的题，next: \"end\" 结束",
		Props:        "{title?, questions: [{id?, question, options: (string | {text, next?, explanation?})[], answer?: number, explanation?, next?}]}",
	},
}

// Builtin 返回仓库内置组件目录，启动时按 Name 播种为 ScopeBuiltin 行；顺序即 SortOrder。
func Builtin() []Component {
	items := make([]Component, 0, len(builtinSpecs))
	for index, spec := range builtinSpecs {
		items = append(items, Component{
			Name:         spec.Name,
			Version:      spec.Version,
			Description:  spec.description(),
			PropsSummary: spec.propsSummary(),
			RendererKind: RendererBuiltin,
			Enabled:      true,
			SortOrder:    index + 1,
		})
	}
	return items
}
