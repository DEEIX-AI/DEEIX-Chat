package channel

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// 模型控件：管理员在能力 JSON 中声明「能力」，用户端只以按钮选择「控制」。
// 用户请求只提交 {控件 id: 选项}，后端据此把管理员预先写好的参数片段（patch）合并进请求，
// 因此用户无法写入任意参数，不再需要参数白名单/黑名单。

// 控件类型。
const (
	ModelControlTypeSelect = "select"
	ModelControlTypeToggle = "toggle"
	ModelControlTypeNumber = "number"
)

// 控件用途：reasoning 为统一思考强度控件，其档位由推理能力翻译为各协议原生参数。
const (
	ModelControlKindGeneric   = "generic"
	ModelControlKindReasoning = "reasoning"
)

// 控件默认位置：toolbar 显示在输入框工具栏，menu 收在「更多」菜单中；用户可在前端调整。
const (
	ModelControlPlacementToolbar = "toolbar"
	ModelControlPlacementMenu    = "menu"
)

// ModelControlReasoningID 是思考强度控件的固定 id。
const ModelControlReasoningID = "reasoning"

// ModelControlReasoningAuto 是 models.dev 目录能力的「自动」档：不下发任何思考参数。
const ModelControlReasoningAuto = "auto"

// 开关控件的两个选项值。
const (
	ModelControlToggleOn  = "on"
	ModelControlToggleOff = "off"
)

const (
	maxModelControls       = 32
	maxModelControlOptions = 32
	maxModelControlLabel   = 64
	maxModelControlDesc    = 240
)

var (
	// ErrInvalidModelControls 表示能力 JSON 中的 controls 声明无效。
	ErrInvalidModelControls = errors.New("invalid model controls")
	// ErrModelControlForbiddenPath 表示控件试图写入由系统管理的参数（模型、消息、流式、凭据、工具等）。
	ErrModelControlForbiddenPath = errors.New("model control writes a forbidden path")

	modelControlIDPattern   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}$`)
	modelControlIconPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,47}$`)
)

// modelControlForbiddenRoots 是控件不得写入的顶层参数：由系统生成或另有专门入口（原生工具）的字段。
var modelControlForbiddenRoots = map[string]struct{}{
	"model": {}, "messages": {}, "input": {}, "instructions": {}, "prompt": {}, "system": {},
	"systemInstruction": {}, "system_instruction": {}, "headers": {}, "api_key": {}, "apiKey": {},
	"base_url": {}, "baseURL": {}, "stream": {}, "stream_options": {}, "previous_response_id": {},
	"prompt_cache_key": {}, "prompt_cache_options": {}, "prompt_cache_breakpoint": {}, "prompt_cache_retention": {},
	"tools": {}, "tool_choice": {}, "functions": {}, "function_call": {}, "contents": {},
}

// ModelControlOption 是 select / toggle 控件的一个选项。
type ModelControlOption struct {
	Value       string
	Label       string
	Description string
	// Patch 为所有协议通用的参数片段；Patches 按协议键覆盖。两者都为空时该选项不修改参数。
	// 片段按对象深度合并，值为 null 表示删除该路径。
	Patch   map[string]any
	Patches map[string]map[string]any
}

// PatchFor 返回选项在协议上的参数片段；协议有专门片段时优先使用。
func (o ModelControlOption) PatchFor(protocolKey string) map[string]any {
	if patch, ok := o.Patches[protocolKey]; ok {
		return patch
	}
	return o.Patch
}

// ModelControl 是一个用户可操作的模型控件。
type ModelControl struct {
	ID          string
	Kind        string
	Type        string
	Label       string
	Description string
	Icon        string
	Placement   string
	// Default 为选项值（select / toggle）或数值的十进制表示（number）；为空表示不修改参数。
	Default string
	// Locked 为 true 时固定使用默认值；Hidden 为 true 时不在用户端展示，但默认值照常生效。
	Locked bool
	Hidden bool
	// Protocols 非空时控件只在这些协议键上生效。
	Protocols []string
	Options   []ModelControlOption
	// Number 控件：写入的参数路径与取值范围。
	Path    string
	Min     *float64
	Max     *float64
	Step    *float64
	Integer bool
	// ReasoningOverride 表示该条只覆盖思考强度控件的展示属性，由推理能力提供档位。
	ReasoningOverride bool
}

// AppliesTo 判断控件是否在协议上生效。
func (c ModelControl) AppliesTo(protocolKey string) bool {
	if len(c.Protocols) == 0 {
		return true
	}
	for _, protocol := range c.Protocols {
		if protocol == protocolKey {
			return true
		}
	}
	return false
}

// AppliesToAny 判断控件是否在任一协议上生效；尚无路由时视为生效。
func (c ModelControl) AppliesToAny(protocolKeys []string) bool {
	if len(c.Protocols) == 0 || len(protocolKeys) == 0 {
		return true
	}
	for _, protocolKey := range protocolKeys {
		if c.AppliesTo(protocolKey) {
			return true
		}
	}
	return false
}

// Option 返回指定值的选项。
func (c ModelControl) Option(value string) (ModelControlOption, bool) {
	for _, option := range c.Options {
		if option.Value == value {
			return option, true
		}
	}
	return ModelControlOption{}, false
}

// NormalizeValue 把用户提交的值收敛为控件可接受的值；无效时返回 false，调用方回退默认值。
func (c ModelControl) NormalizeValue(raw any) (string, bool) {
	switch c.Type {
	case ModelControlTypeSelect:
		value, ok := raw.(string)
		if !ok {
			return "", false
		}
		value = strings.TrimSpace(value)
		if c.Kind == ModelControlKindReasoning {
			value = strings.ToLower(value)
		}
		_, exists := c.Option(value)
		return value, exists
	case ModelControlTypeToggle:
		switch typed := raw.(type) {
		case bool:
			if typed {
				return ModelControlToggleOn, true
			}
			return ModelControlToggleOff, true
		case string:
			switch strings.ToLower(strings.TrimSpace(typed)) {
			case ModelControlToggleOn, "true":
				return ModelControlToggleOn, true
			case ModelControlToggleOff, "false":
				return ModelControlToggleOff, true
			}
		}
		return "", false
	case ModelControlTypeNumber:
		number, ok := modelControlNumber(raw)
		if !ok || !c.acceptsNumber(number) {
			return "", false
		}
		return formatModelControlNumber(number), true
	}
	return "", false
}

// NumberValue 解析 number 控件的规范值。
func (c ModelControl) NumberValue(value string) (float64, bool) {
	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func (c ModelControl) acceptsNumber(number float64) bool {
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return false
	}
	if c.Min != nil && number < *c.Min {
		return false
	}
	if c.Max != nil && number > *c.Max {
		return false
	}
	if c.Integer && number != math.Trunc(number) {
		return false
	}
	if c.Step != nil && *c.Step > 0 {
		base := 0.0
		if c.Min != nil {
			base = *c.Min
		}
		steps := (number - base) / *c.Step
		if math.Abs(steps-math.Round(steps)) > 1e-6 {
			return false
		}
	}
	return true
}

// validateModelControlsOverride 校验能力 JSON 中显式声明的 controls（管理员保存时经 ValidateModelCapsOverrides 调用）；
// 未声明时不干涉。
func validateModelControlsOverride(payload map[string]any) error {
	_, err := parseExplicitModelControls(payload)
	return err
}

// parseExplicitModelControls 解析 capabilities.controls。
func parseExplicitModelControls(payload map[string]any) ([]ModelControl, error) {
	raw, exists := payload["controls"]
	if !exists || raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok || len(items) > maxModelControls {
		return nil, ErrInvalidModelControls
	}
	controls := make([]ModelControl, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		control, err := parseModelControl(item)
		if err != nil {
			return nil, err
		}
		if _, duplicated := seen[control.ID]; duplicated {
			return nil, ErrInvalidModelControls
		}
		seen[control.ID] = struct{}{}
		controls = append(controls, control)
	}
	return controls, nil
}

func parseModelControl(raw any) (ModelControl, error) {
	payload, ok := raw.(map[string]any)
	if !ok {
		return ModelControl{}, ErrInvalidModelControls
	}
	control := ModelControl{
		ID:          modelControlString(payload["id"]),
		Kind:        modelControlString(payload["kind"]),
		Type:        modelControlString(payload["type"]),
		Label:       modelControlString(payload["label"]),
		Description: modelControlString(payload["description"]),
		Icon:        modelControlString(payload["icon"]),
		Placement:   modelControlString(payload["placement"]),
		Path:        modelControlString(payload["path"]),
		Locked:      payload["locked"] == true,
		Hidden:      payload["hidden"] == true,
	}
	if control.Kind == "" {
		control.Kind = ModelControlKindGeneric
	}
	if control.Kind == ModelControlKindReasoning && control.ID == "" {
		control.ID = ModelControlReasoningID
	}
	if !modelControlIDPattern.MatchString(control.ID) ||
		len([]rune(control.Label)) > maxModelControlLabel ||
		len([]rune(control.Description)) > maxModelControlDesc ||
		(control.Icon != "" && !modelControlIconPattern.MatchString(control.Icon)) {
		return ModelControl{}, ErrInvalidModelControls
	}
	switch control.Placement {
	case "":
		// 思考强度控件默认固定到输入框工具栏（见 reasoningModelControl），留空表示「未指定」，
		// 由解析与展示合并决定，避免显式条目把默认位置改成「…」菜单。
		if control.Kind != ModelControlKindReasoning {
			control.Placement = ModelControlPlacementMenu
		}
	case ModelControlPlacementToolbar, ModelControlPlacementMenu:
	default:
		return ModelControl{}, ErrInvalidModelControls
	}
	protocols, err := parseModelControlProtocols(payload["protocols"])
	if err != nil {
		return ModelControl{}, err
	}
	control.Protocols = protocols

	switch control.Kind {
	case ModelControlKindReasoning:
		return parseReasoningModelControl(control, payload)
	case ModelControlKindGeneric:
	default:
		return ModelControl{}, ErrInvalidModelControls
	}
	switch control.Type {
	case ModelControlTypeSelect:
		return parseSelectModelControl(control, payload)
	case ModelControlTypeToggle:
		return parseToggleModelControl(control, payload)
	case ModelControlTypeNumber:
		return parseNumberModelControl(control, payload)
	}
	return ModelControl{}, ErrInvalidModelControls
}

// parseReasoningModelControl 解析思考强度控件：没有 options 时只覆盖展示属性，档位由推理能力提供；
// 有 options 时按 select 解析，选项值必须是规范档位（或 auto）。
func parseReasoningModelControl(control ModelControl, payload map[string]any) (ModelControl, error) {
	control.ID = ModelControlReasoningID
	control.Type = ModelControlTypeSelect
	if _, hasOptions := payload["options"]; !hasOptions {
		// 只覆盖展示属性：未指定 placement 时沿用自动生成控件的默认位置。
		control.ReasoningOverride = true
		control.Default = ""
		return control, nil
	}
	parsed, err := parseSelectModelControl(control, payload)
	if err != nil {
		return ModelControl{}, err
	}
	if parsed.Placement == "" {
		parsed.Placement = ModelControlPlacementToolbar
	}
	for _, option := range parsed.Options {
		if option.Value != ModelControlReasoningAuto && ReasoningLevelRank(option.Value) < 0 {
			return ModelControl{}, ErrInvalidModelControls
		}
	}
	return parsed, nil
}

func parseSelectModelControl(control ModelControl, payload map[string]any) (ModelControl, error) {
	items, ok := payload["options"].([]any)
	if !ok || len(items) == 0 || len(items) > maxModelControlOptions {
		return ModelControl{}, ErrInvalidModelControls
	}
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		option, err := parseModelControlOption(item, control.Kind == ModelControlKindReasoning)
		if err != nil {
			return ModelControl{}, err
		}
		if _, duplicated := seen[option.Value]; duplicated {
			return ModelControl{}, ErrInvalidModelControls
		}
		seen[option.Value] = struct{}{}
		control.Options = append(control.Options, option)
	}
	if raw, exists := payload["default"]; exists && raw != nil {
		value, ok := control.NormalizeValue(raw)
		if !ok {
			return ModelControl{}, ErrInvalidModelControls
		}
		control.Default = value
	}
	return control, nil
}

func parseToggleModelControl(control ModelControl, payload map[string]any) (ModelControl, error) {
	on, err := parseModelControlPatchSet(payload["on"])
	if err != nil {
		return ModelControl{}, err
	}
	off, err := parseModelControlPatchSet(payload["off"])
	if err != nil {
		return ModelControl{}, err
	}
	on.Value, off.Value = ModelControlToggleOn, ModelControlToggleOff
	control.Options = []ModelControlOption{off, on}
	control.Default = ModelControlToggleOff
	if raw, exists := payload["default"]; exists && raw != nil {
		value, ok := control.NormalizeValue(raw)
		if !ok {
			return ModelControl{}, ErrInvalidModelControls
		}
		control.Default = value
	}
	return control, nil
}

func parseNumberModelControl(control ModelControl, payload map[string]any) (ModelControl, error) {
	if err := validateModelControlPath(control.Path); err != nil {
		return ModelControl{}, err
	}
	for key, target := range map[string]**float64{"min": &control.Min, "max": &control.Max, "step": &control.Step} {
		if raw, exists := payload[key]; exists && raw != nil {
			number, ok := modelControlNumber(raw)
			if !ok {
				return ModelControl{}, ErrInvalidModelControls
			}
			*target = &number
		}
	}
	control.Integer = payload["integer"] == true
	if control.Min != nil && control.Max != nil && *control.Min > *control.Max {
		return ModelControl{}, ErrInvalidModelControls
	}
	if control.Step != nil && *control.Step <= 0 {
		return ModelControl{}, ErrInvalidModelControls
	}
	if raw, exists := payload["default"]; exists && raw != nil {
		value, ok := control.NormalizeValue(raw)
		if !ok {
			return ModelControl{}, ErrInvalidModelControls
		}
		control.Default = value
	}
	return control, nil
}

func parseModelControlOption(raw any, reasoning bool) (ModelControlOption, error) {
	payload, ok := raw.(map[string]any)
	if !ok {
		return ModelControlOption{}, ErrInvalidModelControls
	}
	option, err := parseModelControlPatchSet(payload)
	if err != nil {
		return ModelControlOption{}, err
	}
	option.Value = modelControlString(payload["value"])
	if reasoning {
		option.Value = strings.ToLower(option.Value)
	}
	if option.Value == "" || len(option.Value) > 64 {
		return ModelControlOption{}, ErrInvalidModelControls
	}
	return option, nil
}

// parseModelControlPatchSet 解析选项的 label / description / patch / patches。
func parseModelControlPatchSet(raw any) (ModelControlOption, error) {
	if raw == nil {
		return ModelControlOption{}, nil
	}
	payload, ok := raw.(map[string]any)
	if !ok {
		return ModelControlOption{}, ErrInvalidModelControls
	}
	option := ModelControlOption{
		Label:       modelControlString(payload["label"]),
		Description: modelControlString(payload["description"]),
	}
	if len([]rune(option.Label)) > maxModelControlLabel || len([]rune(option.Description)) > maxModelControlDesc {
		return ModelControlOption{}, ErrInvalidModelControls
	}
	if rawPatch, exists := payload["patch"]; exists && rawPatch != nil {
		patch, ok := rawPatch.(map[string]any)
		if !ok {
			return ModelControlOption{}, ErrInvalidModelControls
		}
		if err := validateModelControlPatch(patch); err != nil {
			return ModelControlOption{}, err
		}
		option.Patch = patch
	}
	if rawPatches, exists := payload["patches"]; exists && rawPatches != nil {
		patches, ok := rawPatches.(map[string]any)
		if !ok {
			return ModelControlOption{}, ErrInvalidModelControls
		}
		option.Patches = make(map[string]map[string]any, len(patches))
		for protocol, rawPatch := range patches {
			patch, ok := rawPatch.(map[string]any)
			if !ok || strings.TrimSpace(protocol) == "" {
				return ModelControlOption{}, ErrInvalidModelControls
			}
			if err := validateModelControlPatch(patch); err != nil {
				return ModelControlOption{}, err
			}
			option.Patches[strings.TrimSpace(protocol)] = patch
		}
	}
	return option, nil
}

func parseModelControlProtocols(raw any) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, ErrInvalidModelControls
	}
	protocols := make([]string, 0, len(items))
	for _, item := range items {
		protocol := modelControlString(item)
		if protocol == "" {
			return nil, ErrInvalidModelControls
		}
		protocols = append(protocols, protocol)
	}
	return protocols, nil
}

func validateModelControlPatch(patch map[string]any) error {
	for key := range patch {
		if _, forbidden := modelControlForbiddenRoots[strings.TrimSpace(key)]; forbidden {
			return ErrModelControlForbiddenPath
		}
	}
	return nil
}

func validateModelControlPath(path string) error {
	segments := strings.Split(path, ".")
	for _, segment := range segments {
		if strings.TrimSpace(segment) == "" {
			return ErrInvalidModelControls
		}
	}
	if _, forbidden := modelControlForbiddenRoots[strings.TrimSpace(segments[0])]; forbidden {
		return ErrModelControlForbiddenPath
	}
	return nil
}

// ---------------------------------------------------------------------------
// 生效控件
// ---------------------------------------------------------------------------

// ResolveModelControls 汇总模型在可用协议上的生效控件，顺序为：思考强度（可被显式条目重新定位）、
// 显式 controls、由旧版 optionControls 编译的控件。reasoning 为已解析的推理能力（可为 nil），
// reasoningSource 为其来源；目录来源的思考强度控件带「自动」档并默认选中。
func ResolveModelControls(capabilitiesJSON string, protocolKeys []string, reasoning *ReasoningCapability, reasoningSource string) []ModelControl {
	payload, _ := parseCapabilities(capabilitiesJSON)
	explicit, err := parseExplicitModelControls(payload)
	if err != nil {
		explicit = nil
	}
	lockedPaths := modelControlLockedPaths(payload)

	reasoningControl, hasReasoning := reasoningModelControl(reasoning, reasoningSource, lockedPaths, protocolKeys)
	controls := make([]ModelControl, 0, len(explicit)+4)
	covered := map[string]struct{}{}
	placedReasoning := false
	for _, control := range explicit {
		if control.Kind == ModelControlKindReasoning {
			if control.ReasoningOverride {
				if !hasReasoning {
					continue
				}
				control = mergeReasoningPresentation(reasoningControl, control)
			}
			placedReasoning = true
		}
		if !control.AppliesToAny(protocolKeys) {
			continue
		}
		covered[control.ID] = struct{}{}
		if control.Path != "" {
			covered[control.Path] = struct{}{}
		}
		controls = append(controls, control)
	}
	if hasReasoning && !placedReasoning {
		controls = append([]ModelControl{reasoningControl}, controls...)
	}
	reasoningPaths := map[string]struct{}{}
	if hasReasoning || placedReasoning {
		for _, protocolKey := range protocolKeys {
			if path := legacyReasoningControlPath(protocolKey); path != "" {
				reasoningPaths[path] = struct{}{}
			}
		}
		if reasoning != nil && reasoning.ControlPath != "" {
			reasoningPaths[reasoning.ControlPath] = struct{}{}
		}
	}
	for _, control := range compileLegacyModelControls(payload, lockedPaths) {
		if _, skip := covered[control.ID]; skip {
			continue
		}
		if _, skip := reasoningPaths[control.ID]; skip {
			continue
		}
		covered[control.ID] = struct{}{}
		controls = append(controls, control)
	}
	return controls
}

// reasoningModelControl 由推理能力生成思考强度控件。
func reasoningModelControl(capability *ReasoningCapability, source string, lockedPaths map[string]struct{}, protocolKeys []string) (ModelControl, bool) {
	if capability == nil {
		return ModelControl{}, false
	}
	control := ModelControl{
		ID:        ModelControlReasoningID,
		Kind:      ModelControlKindReasoning,
		Type:      ModelControlTypeSelect,
		Icon:      "brain",
		Placement: ModelControlPlacementToolbar,
		Default:   capability.Default,
	}
	if _, locked := lockedPaths[ModelControlReasoningID]; locked {
		control.Locked = true
	}
	if _, locked := lockedPaths["reasoning_effort"]; locked {
		control.Locked = true
	}
	for _, protocolKey := range protocolKeys {
		if path := ReasoningNativeLevelPath(capability.Format, protocolKey); path != "" {
			if _, locked := lockedPaths[path]; locked {
				control.Locked = true
			}
		}
	}
	if capability.ControlPath != "" {
		if _, locked := lockedPaths[capability.ControlPath]; locked {
			control.Locked = true
		}
	}
	if source == ReasoningSourceCatalog && !control.Locked {
		control.Options = append(control.Options, ModelControlOption{Value: ModelControlReasoningAuto})
		control.Default = ModelControlReasoningAuto
	}
	for _, level := range capability.Levels {
		control.Options = append(control.Options, ModelControlOption{Value: level})
	}
	return control, true
}

// mergeReasoningPresentation 用显式条目的展示属性覆盖自动生成的思考强度控件。
func mergeReasoningPresentation(base ModelControl, override ModelControl) ModelControl {
	if override.Label != "" {
		base.Label = override.Label
	}
	if override.Description != "" {
		base.Description = override.Description
	}
	if override.Icon != "" {
		base.Icon = override.Icon
	}
	// 未指定 placement 的显式条目不改变默认位置。
	if override.Placement != "" {
		base.Placement = override.Placement
	}
	base.Locked = base.Locked || override.Locked
	base.Hidden = override.Hidden
	base.Protocols = override.Protocols
	return base
}

// compileLegacyModelControls 把旧版 optionControls 编译为控件：select → select（选项值写入 path），
// boolean → toggle，number → number。text / json 等自由输入控件只对管理员开放，不编译。
func compileLegacyModelControls(payload map[string]any, lockedPaths map[string]struct{}) []ModelControl {
	items, _ := payload["optionControls"].([]any)
	defaults, _ := payload["defaultOptions"].(map[string]any)
	controls := make([]ModelControl, 0, len(items))
	for _, item := range items {
		raw, ok := item.(map[string]any)
		if !ok {
			continue
		}
		path := modelControlString(raw["path"])
		if path == "" || !modelControlIDPattern.MatchString(path) || validateModelControlPath(path) != nil {
			continue
		}
		control := ModelControl{
			ID:          path,
			Kind:        ModelControlKindGeneric,
			Label:       modelControlString(raw["label"]),
			Description: modelControlString(raw["description"]),
			Placement:   ModelControlPlacementMenu,
			Locked:      raw["locked"] == true,
		}
		if _, locked := lockedPaths[path]; locked {
			control.Locked = true
		}
		segments := strings.Split(path, ".")
		defaultValue, hasDefault := readCapabilityPath(defaults, segments)
		switch strings.ToLower(modelControlString(raw["type"])) {
		case ModelControlTypeSelect:
			values, _ := raw["options"].([]any)
			numeric := false
			if _, isString := defaultValue.(string); hasDefault && !isString {
				_, numeric = modelControlNumber(defaultValue)
			}
			for _, rawValue := range values {
				text := strings.TrimSpace(modelControlScalarString(rawValue))
				if text == "" {
					continue
				}
				var value any = text
				if numeric {
					if number, ok := modelControlNumber(text); ok {
						value = number
					}
				}
				control.Options = append(control.Options, ModelControlOption{Value: text, Patch: modelControlPathPatch(segments, value)})
			}
			if len(control.Options) == 0 {
				continue
			}
			control.Type = ModelControlTypeSelect
			if hasDefault {
				if _, exists := control.Option(modelControlScalarString(defaultValue)); exists {
					control.Default = modelControlScalarString(defaultValue)
				}
			}
		case "boolean", ModelControlTypeToggle:
			control.Type = ModelControlTypeToggle
			control.Options = []ModelControlOption{
				{Value: ModelControlToggleOff, Patch: modelControlPathPatch(segments, false)},
				{Value: ModelControlToggleOn, Patch: modelControlPathPatch(segments, true)},
			}
			if enabled, ok := defaultValue.(bool); hasDefault && ok {
				control.Default = ModelControlToggleOff
				if enabled {
					control.Default = ModelControlToggleOn
				}
			}
		case ModelControlTypeNumber:
			control.Type = ModelControlTypeNumber
			control.Path = path
			if hasDefault {
				if number, ok := modelControlNumber(defaultValue); ok {
					control.Default = formatModelControlNumber(number)
				}
			}
		default:
			continue
		}
		controls = append(controls, control)
	}
	return controls
}

func modelControlLockedPaths(payload map[string]any) map[string]struct{} {
	result := map[string]struct{}{}
	items, _ := payload["lockedOptionPaths"].([]any)
	for _, item := range items {
		if path := modelControlString(item); path != "" {
			result[path] = struct{}{}
		}
	}
	return result
}

func modelControlPathPatch(segments []string, value any) map[string]any {
	patch := map[string]any{}
	current := patch
	for index, segment := range segments {
		if index == len(segments)-1 {
			current[segment] = value
			break
		}
		next := map[string]any{}
		current[segment] = next
		current = next
	}
	return patch
}

func modelControlString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func modelControlScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	}
	if number, ok := modelControlNumber(value); ok {
		return formatModelControlNumber(number)
	}
	return ""
}

func modelControlNumber(value any) (float64, bool) {
	var number float64
	switch typed := value.(type) {
	case float64:
		number = typed
	case int:
		number = float64(typed)
	case int64:
		number = float64(typed)
	case interface{ Float64() (float64, error) }:
		parsed, err := typed.Float64()
		if err != nil {
			return 0, false
		}
		number = parsed
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return 0, false
		}
		number = parsed
	default:
		return 0, false
	}
	if math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false
	}
	return number, true
}

func formatModelControlNumber(number float64) string {
	return strconv.FormatFloat(number, 'f', -1, 64)
}
