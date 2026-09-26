package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 本地模式：服务器作为桌面应用的 sidecar 运行，只服务这台机器上的这一个用户。
//
// 设计原则是"和服务器部署使用完全相同的代码与安全策略，只把基础设施换成本地资源"：
//   - 存储：SQLite + sqlite-vec + 进程内缓存 + 本地文件目录，全部落在 dataDir 下
//   - 网络：只监听 127.0.0.1，端口由操作系统分配，父进程通过握手拿到实际端口
//   - 密钥：JWT 密钥与数据加密密钥按安装实例生成一次，0600 存于 dataDir，
//     因此 Env 保持 prod，生产级校验（密钥强度、CORS 白名单）全部生效
//   - 出站：关闭 GeoIP 等在本地毫无意义的外部查询
// 本地模式不新增任何绕过鉴权的路径；唯一的附加能力是一次性的本地登录 grant，
// 只有拉起 sidecar 的父进程能拿到（见 application/auth 的 LocalGrant）。

const (
	// LocalListenAddr 本地模式固定绑定回环地址，端口 0 表示由系统分配。
	LocalListenAddr = "127.0.0.1:0"
	// localSecretsFile 存放按安装实例生成的密钥。
	localSecretsFile = "secrets.json"
	// localOwnerUsername 本地模式唯一用户的用户名；刻意与 AdminUsername 不同，
	// 以免触发"首次登录必须改用户名/改密码"的初始化引导。
	localOwnerUsername = "owner"
)

// LocalSecrets 是按安装实例生成、持久化在数据目录的密钥。
type LocalSecrets struct {
	JWTSecret         string `json:"jwtSecret"`
	DataEncryptionKey string `json:"dataEncryptionKey"`
}

// ApplyLocalMode 把配置切换到本地模式。dataDir 必须是绝对路径；不存在时创建。
func (c *Config) ApplyLocalMode(dataDir string) error {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" || !filepath.IsAbs(dataDir) {
		return errors.New("local mode: data directory must be an absolute path")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("local mode: create data directory: %w", err)
	}
	secrets, err := loadOrCreateLocalSecrets(filepath.Join(dataDir, localSecretsFile))
	if err != nil {
		return err
	}

	c.LocalMode = true
	c.LocalDataDir = dataDir
	c.Env = "prod"
	c.JWTSecret = secrets.JWTSecret
	c.DataEncryptionKey = secrets.DataEncryptionKey

	c.HTTPListenAddr = LocalListenAddr
	c.TrustedProxies = ""
	// 公共地址在监听成功后由 SetLocalOrigin 填入实际端口。
	c.PublicAPIBaseURL = ""
	c.PublicWebBaseURL = ""
	// 桌面 webview 的 Origin；本地开发时 next dev 也会以 localhost:3000 访问。
	c.CORSAllowOrigin = "tauri://localhost,http://tauri.localhost,http://localhost:3000,http://127.0.0.1:3000"
	// 前端由桌面壳自己承载，服务器不再提供静态文件。
	c.FrontendDistDir = ""

	c.DatabaseDriver = "sqlite"
	c.SQLiteDSN = ""
	c.SQLitePath = filepath.Join(dataDir, "deeix.db")
	c.CacheDriver = "memory"
	c.StorageBackend = "local"
	c.StorageRootDir = filepath.Join(dataDir, "storage")

	c.GeoIPProvider = "none"
	c.AdminUsername = localOwnerUsername
	return nil
}

// SetLocalOrigin 在监听成功后写入实际的回环 origin，供分享链接、OAuth 回调等使用。
func (c *Config) SetLocalOrigin(origin string) {
	c.PublicAPIBaseURL = origin
	c.PublicWebBaseURL = origin
}

func loadOrCreateLocalSecrets(path string) (*LocalSecrets, error) {
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		var secrets LocalSecrets
		if err := json.Unmarshal(raw, &secrets); err != nil {
			return nil, fmt.Errorf("local mode: secrets file %s is corrupt: %w", path, err)
		}
		if len(secrets.JWTSecret) < 32 || len(secrets.DataEncryptionKey) < 32 {
			return nil, fmt.Errorf("local mode: secrets file %s is incomplete; delete it to regenerate (existing encrypted data will be unreadable)", path)
		}
		return &secrets, nil
	case errors.Is(err, os.ErrNotExist):
		secrets := &LocalSecrets{JWTSecret: randomHex(32), DataEncryptionKey: randomHex(32)}
		body, err := json.MarshalIndent(secrets, "", "  ")
		if err != nil {
			return nil, err
		}
		// 先写临时文件再改名，避免半写入的密钥文件被下次启动读到。
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, body, 0o600); err != nil {
			return nil, fmt.Errorf("local mode: write secrets: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return nil, fmt.Errorf("local mode: commit secrets: %w", err)
		}
		return secrets, nil
	default:
		return nil, fmt.Errorf("local mode: read secrets: %w", err)
	}
}

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(buf)
}
