package settings

// SettingItem 单个配置项内部传输结构，不携带序列化标记。
type SettingItem struct {
	Key         string
	Value       string
	ValueType   string
	Description string
	Sensitive   bool
	Configured  bool
}

// PatchItem 单个更新项。
type PatchItem struct {
	Namespace string
	Key       string
	Value     string
	Clear     bool
}
