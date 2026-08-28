package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/extraction"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
)

const (
	tikaContainerName      = "deeix-chat-tika"
	tikaImage              = "apache/tika:3.2.3.0"
	tesseractContainerName = "deeix-chat-tesseract"
	tesseractImage         = "deeix-chat-tesseract:latest"
	rapidOCRContainerName  = "deeix-chat-rapidocr"
	rapidOCRImage          = "deeix-chat-rapidocr:latest"
	doclingContainerName   = "deeix-chat-docling"
	doclingImage           = "deeix-chat-docling:latest"
	serviceNetwork         = "deeix-chat-network"
	dockerDefaultTimeout   = 3 * time.Minute
	dockerBuildTimeout     = 3 * time.Minute
)

// ServiceRuntimeView 表示受管服务运行状态。
type ServiceRuntimeView struct {
	Source        string
	BaseURL       string
	ContainerName string
	Image         string
	Network       string
	Status        string
	Reachable     bool
	Message       string
}

// Service 提供可选依赖服务的托管能力。
type Service struct {
	cfg          *config.Runtime
	dockerRunner DockerRunner
	prober       EngineProber
}

// DockerRunner 是应用层所需的 Docker 命令能力。
type DockerRunner interface {
	Available() bool
	RunWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error)
}

// EngineProber 探测抽取引擎端点可用性并解析托管端点地址。
type EngineProber interface {
	ProbeTika(ctx context.Context, baseURL, authToken string) (reachable bool, message string)
	ResolveManagedTikaBaseURL(ctx context.Context) string
	ProbeDocling(ctx context.Context, baseURL, authToken string) (reachable bool, message string)
	ProbeMinerU(ctx context.Context, baseURL, authToken string) (reachable bool, message string)
	ProbeOCR(ctx context.Context, baseURL, authToken string) (reachable bool, message string)
	ProbeRapidOCR(ctx context.Context, baseURL, authToken string) (reachable bool, message string)
	ResolveManagedRapidOCRBaseURL(ctx context.Context) string
}

// NewService 创建运行时服务管理器。
func NewService(cfg *config.Runtime, prober EngineProber) *Service {
	return &Service{cfg: cfg, prober: prober}
}

// SetDockerRunner 注入 Docker 命令执行器。
func (s *Service) SetDockerRunner(runner DockerRunner) {
	s.dockerRunner = runner
}

func (s *Service) dockerAvailable() bool {
	return s != nil && s.dockerRunner != nil && s.dockerRunner.Available()
}

// GetTikaStatus 查询当前 Tika 服务状态。
func (s *Service) GetTikaStatus(ctx context.Context) ServiceRuntimeView {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	source := extraction.NormalizeTikaSourceForRuntime(snapshot.ExtractTikaSource)
	baseURL := strings.TrimSpace(snapshot.ExtractTikaBaseURL)
	if source == extraction.TikaSourceManaged || baseURL == "" {
		baseURL = s.prober.ResolveManagedTikaBaseURL(ctx)
	}

	view := ServiceRuntimeView{
		Source:        source,
		BaseURL:       baseURL,
		ContainerName: tikaContainerName,
		Image:         tikaImage,
		Network:       serviceNetwork,
	}

	if source == extraction.TikaSourceExternal {
		if strings.TrimSpace(snapshot.ExtractTikaBaseURL) == "" {
			view.Status = "unconfigured"
			view.Message = "请先填写 Tika 服务地址。"
			return view
		}
		reachable, message := s.prober.ProbeTika(ctx, baseURL, snapshot.ExtractTikaAuthToken)
		view.Reachable = reachable
		if reachable {
			view.Status = "running"
			view.Message = "已连接到外部 Tika 服务。"
		} else {
			view.Status = "unhealthy"
			view.Message = message
		}
		return view
	}

	if !s.dockerAvailable() {
		view.Status = "unavailable"
		view.Message = "当前环境未检测到 docker 命令，无法托管 Tika。"
		return view
	}

	containerStatus, exists, err := s.inspectContainerStatus(ctx, tikaContainerName)
	if err != nil {
		view.Status = "failed"
		view.Message = err.Error()
		return view
	}
	if !exists {
		view.Status = "stopped"
		view.Message = "Tika 容器尚未启动。"
		return view
	}

	view.Status = containerStatus
	reachable, message := s.prober.ProbeTika(ctx, baseURL, "")
	view.Reachable = reachable
	if containerStatus == "running" {
		if reachable {
			view.Status = "running"
			view.Message = "Tika 容器运行正常。"
		} else {
			view.Status = "unhealthy"
			view.Message = message
		}
		return view
	}

	if message != "" {
		view.Message = message
	} else {
		view.Message = "Tika 容器当前未运行。"
	}
	return view
}

// StartTika 启动托管的 Tika 服务。
func (s *Service) StartTika(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedTika(); err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetTikaStatus(ctx), fmt.Errorf("docker_not_available")
	}
	if err := s.ensureDockerNetwork(ctx, serviceNetwork); err != nil {
		return s.GetTikaStatus(ctx), err
	}

	status, exists, err := s.inspectContainerStatus(ctx, tikaContainerName)
	if err != nil {
		return s.GetTikaStatus(ctx), err
	}
	switch {
	case !exists:
		if _, err = s.runDocker(ctx,
			"run", "-d",
			"--name", tikaContainerName,
			"--network", serviceNetwork,
			"-p", "127.0.0.1:9998:9998",
			"--restart", "unless-stopped",
			tikaImage,
		); err != nil {
			return s.GetTikaStatus(ctx), err
		}
	case status != "running":
		if _, err = s.runDocker(ctx, "start", tikaContainerName); err != nil {
			return s.GetTikaStatus(ctx), err
		}
	}

	if err := s.waitForManagedReachable(ctx, 20*time.Second); err != nil {
		view := s.GetTikaStatus(ctx)
		if strings.TrimSpace(view.Message) == "" {
			view.Message = err.Error()
		}
		return view, err
	}
	return s.GetTikaStatus(ctx), nil
}

// StopTika 停止托管的 Tika 服务。
func (s *Service) StopTika(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedTika(); err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetTikaStatus(ctx), fmt.Errorf("docker_not_available")
	}

	_, exists, err := s.inspectContainerStatus(ctx, tikaContainerName)
	if err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if exists {
		if err = s.stopAndRemoveContainer(ctx, tikaContainerName); err != nil {
			return s.GetTikaStatus(ctx), err
		}
	}
	return s.GetTikaStatus(ctx), nil
}

// RestartTika 重启托管的 Tika 服务。
func (s *Service) RestartTika(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedTika(); err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetTikaStatus(ctx), fmt.Errorf("docker_not_available")
	}

	_, exists, err := s.inspectContainerStatus(ctx, tikaContainerName)
	if err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if !exists {
		return s.StartTika(ctx)
	}
	if _, err = s.runDocker(ctx, "restart", tikaContainerName); err != nil {
		return s.GetTikaStatus(ctx), err
	}
	if err := s.waitForManagedReachable(ctx, 20*time.Second); err != nil {
		view := s.GetTikaStatus(ctx)
		if strings.TrimSpace(view.Message) == "" {
			view.Message = err.Error()
		}
		return view, err
	}
	return s.GetTikaStatus(ctx), nil
}

func (s *Service) requireManagedTika() error {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}
	if extraction.NormalizeTikaSourceForRuntime(snapshot.ExtractTikaSource) != extraction.TikaSourceManaged {
		return fmt.Errorf("tika_not_managed")
	}
	return nil
}

// GetRapidOCRStatus 查询当前 RapidOCR 服务状态。
func (s *Service) GetRapidOCRStatus(ctx context.Context) ServiceRuntimeView {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	source := extraction.NormalizeTikaSourceForRuntime(snapshot.ExtractRapidOCRSource)
	baseURL := strings.TrimSpace(snapshot.ExtractRapidOCRBaseURL)
	if source == extraction.TikaSourceManaged || baseURL == "" {
		baseURL = s.prober.ResolveManagedRapidOCRBaseURL(ctx)
	}

	view := ServiceRuntimeView{
		Source:        source,
		BaseURL:       baseURL,
		ContainerName: rapidOCRContainerName,
		Image:         rapidOCRImage,
		Network:       serviceNetwork,
	}

	if source == extraction.TikaSourceExternal {
		if strings.TrimSpace(snapshot.ExtractRapidOCRBaseURL) == "" {
			view.Status = "unconfigured"
			view.Message = "请先填写 RapidOCR 服务地址。"
			return view
		}
		reachable, message := s.prober.ProbeRapidOCR(ctx, baseURL, snapshot.ExtractRapidOCRAuthToken)
		view.Reachable = reachable
		if reachable {
			view.Status = "running"
			view.Message = "已连接到外部 RapidOCR 服务。"
		} else {
			view.Status = "unhealthy"
			view.Message = message
		}
		return view
	}

	if !s.dockerAvailable() {
		view.Status = "unavailable"
		view.Message = "当前环境未检测到 docker 命令，无法托管 RapidOCR。"
		return view
	}

	containerStatus, exists, err := s.inspectContainerStatus(ctx, rapidOCRContainerName)
	if err != nil {
		view.Status = "failed"
		view.Message = err.Error()
		return view
	}
	if !exists {
		view.Status = "stopped"
		view.Message = "RapidOCR 容器尚未启动。"
		return view
	}

	view.Status = containerStatus
	reachable, message := s.prober.ProbeRapidOCR(ctx, baseURL, "")
	view.Reachable = reachable
	if containerStatus == "running" {
		if reachable {
			view.Status = "running"
			view.Message = "RapidOCR 容器运行正常。"
		} else {
			view.Status = "unhealthy"
			view.Message = message
		}
		return view
	}

	if message != "" {
		view.Message = message
	} else {
		view.Message = "RapidOCR 容器当前未运行。"
	}
	return view
}

// GetDoclingStatus 查询当前 Docling 服务状态。
func (s *Service) GetDoclingStatus(ctx context.Context) ServiceRuntimeView {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	baseURL := strings.TrimSpace(snapshot.ExtractDoclingBaseURL)
	view := ServiceRuntimeView{
		Source:        extraction.TikaSourceExternal,
		BaseURL:       baseURL,
		ContainerName: doclingContainerName,
		Image:         doclingImage,
		Network:       serviceNetwork,
	}
	if baseURL == "" {
		view.Status = "unconfigured"
		view.Message = "请先填写 Docling 服务地址。"
		return view
	}
	reachable, message := s.prober.ProbeDocling(ctx, baseURL, snapshot.ExtractDoclingAuthToken)
	view.Reachable = reachable
	if reachable {
		view.Status = "running"
		view.Message = "已连接到外部 Docling 服务。"
	} else {
		view.Status = "unhealthy"
		view.Message = message
	}
	return view
}

// GetTesseractStatus 查询当前 Tesseract OCR 服务状态。
func (s *Service) GetTesseractStatus(ctx context.Context) ServiceRuntimeView {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	baseURL := strings.TrimSpace(snapshot.ExtractTesseractOCRBaseURL)
	view := ServiceRuntimeView{
		Source:        extraction.TikaSourceExternal,
		BaseURL:       baseURL,
		ContainerName: tesseractContainerName,
		Image:         tesseractImage,
		Network:       serviceNetwork,
	}

	if baseURL == "" {
		view.Status = "unconfigured"
		view.Message = "请先填写 Tesseract OCR 服务地址。"
		return view
	}

	reachable, message := s.prober.ProbeOCR(ctx, baseURL, snapshot.ExtractTesseractOCRAuthToken)
	view.Reachable = reachable
	if reachable {
		view.Status = "running"
		view.Message = "已连接到外部 Tesseract OCR 服务。"
	} else {
		view.Status = "unhealthy"
		view.Message = message
	}
	return view
}

// GetMinerUStatus 查询当前 MinerU 服务状态。
func (s *Service) GetMinerUStatus(ctx context.Context) ServiceRuntimeView {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}

	baseURL := strings.TrimSpace(snapshot.ExtractMinerUBaseURL)
	view := ServiceRuntimeView{
		Source:        strings.TrimSpace(snapshot.ExtractMinerUSource),
		BaseURL:       baseURL,
		ContainerName: "",
		Image:         "",
		Network:       "",
	}
	if baseURL == "" {
		view.Status = "unconfigured"
		view.Message = "请先填写 MinerU 服务地址。"
		return view
	}
	reachable, message := s.prober.ProbeMinerU(ctx, baseURL, snapshot.ExtractMinerUAuthToken)
	view.Reachable = reachable
	if reachable {
		view.Status = "running"
		view.Message = "已连接到外部 MinerU 服务。"
	} else {
		view.Status = "unhealthy"
		view.Message = message
	}
	return view
}

// StartRapidOCR 启动托管的 RapidOCR 服务。
func (s *Service) StartRapidOCR(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedRapidOCR(); err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetRapidOCRStatus(ctx), fmt.Errorf("docker_not_available")
	}
	if err := s.ensureDockerNetwork(ctx, serviceNetwork); err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if _, err := s.runDocker(ctx, "image", "inspect", rapidOCRImage); err != nil {
		if _, buildErr := s.runDockerWithTimeout(ctx, dockerBuildTimeout, "build", "-t", rapidOCRImage, "-f", "../docker/rapidocr/Dockerfile", ".."); buildErr != nil {
			return s.GetRapidOCRStatus(ctx), buildErr
		}
	}

	status, exists, err := s.inspectContainerStatus(ctx, rapidOCRContainerName)
	if err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	switch {
	case !exists:
		if _, err = s.runDocker(ctx,
			"run", "-d",
			"--name", rapidOCRContainerName,
			"--network", serviceNetwork,
			"-p", "127.0.0.1:8002:8002",
			"--restart", "unless-stopped",
			rapidOCRImage,
		); err != nil {
			return s.GetRapidOCRStatus(ctx), err
		}
	case status != "running":
		if _, err = s.runDocker(ctx, "start", rapidOCRContainerName); err != nil {
			return s.GetRapidOCRStatus(ctx), err
		}
	}

	if err := s.waitForManagedRapidOCRReachable(ctx, 20*time.Second); err != nil {
		view := s.GetRapidOCRStatus(ctx)
		if strings.TrimSpace(view.Message) == "" {
			view.Message = err.Error()
		}
		return view, err
	}
	return s.GetRapidOCRStatus(ctx), nil
}

// StopRapidOCR 停止托管的 RapidOCR 服务。
func (s *Service) StopRapidOCR(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedRapidOCR(); err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetRapidOCRStatus(ctx), fmt.Errorf("docker_not_available")
	}
	_, exists, err := s.inspectContainerStatus(ctx, rapidOCRContainerName)
	if err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if exists {
		if err = s.stopAndRemoveContainer(ctx, rapidOCRContainerName); err != nil {
			return s.GetRapidOCRStatus(ctx), err
		}
	}
	return s.GetRapidOCRStatus(ctx), nil
}

// RestartRapidOCR 重启托管的 RapidOCR 服务。
func (s *Service) RestartRapidOCR(ctx context.Context) (ServiceRuntimeView, error) {
	if err := s.requireManagedRapidOCR(); err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if !s.dockerAvailable() {
		return s.GetRapidOCRStatus(ctx), fmt.Errorf("docker_not_available")
	}
	_, exists, err := s.inspectContainerStatus(ctx, rapidOCRContainerName)
	if err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if !exists {
		return s.StartRapidOCR(ctx)
	}
	if _, err = s.runDocker(ctx, "restart", rapidOCRContainerName); err != nil {
		return s.GetRapidOCRStatus(ctx), err
	}
	if err := s.waitForManagedRapidOCRReachable(ctx, 20*time.Second); err != nil {
		view := s.GetRapidOCRStatus(ctx)
		if strings.TrimSpace(view.Message) == "" {
			view.Message = err.Error()
		}
		return view, err
	}
	return s.GetRapidOCRStatus(ctx), nil
}

func (s *Service) requireManagedRapidOCR() error {
	snapshot := config.Config{}
	if s != nil && s.cfg != nil {
		snapshot = s.cfg.Snapshot()
	}
	if extraction.NormalizeTikaSourceForRuntime(snapshot.ExtractRapidOCRSource) != extraction.TikaSourceManaged {
		return fmt.Errorf("rapidocr_not_managed")
	}
	return nil
}

func (s *Service) ensureDockerNetwork(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("docker_network_invalid")
	}
	if _, err := s.runDocker(ctx, "network", "inspect", name); err == nil {
		return nil
	}
	_, err := s.runDocker(ctx, "network", "create", name)
	return err
}

func (s *Service) inspectContainerStatus(ctx context.Context, name string) (string, bool, error) {
	output, err := s.runDocker(ctx, "container", "inspect", "--format", "{{.State.Status}}", name)
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "no such object") || strings.Contains(lower, "error: no such object") || strings.Contains(lower, "no such container") {
			return "", false, nil
		}
		return "", false, err
	}
	return strings.TrimSpace(output), true, nil
}

func (s *Service) stopAndRemoveContainer(ctx context.Context, name string) error {
	status, exists, err := s.inspectContainerStatus(ctx, name)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if status == "running" || status == "restarting" || status == "paused" {
		if _, err = s.runDocker(ctx, "stop", name); err != nil {
			return err
		}
	}
	if _, err = s.runDocker(ctx, "rm", "-f", name); err != nil {
		return err
	}
	return nil
}

func (s *Service) runDocker(ctx context.Context, args ...string) (string, error) {
	return s.runDockerWithTimeout(ctx, dockerDefaultTimeout, args...)
}

func (s *Service) runDockerWithTimeout(ctx context.Context, timeout time.Duration, args ...string) (string, error) {
	if s == nil || s.dockerRunner == nil {
		return "", fmt.Errorf("docker_not_available")
	}
	return s.dockerRunner.RunWithTimeout(ctx, timeout, args...)
}

func (s *Service) waitForManagedReachable(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		baseURL := s.prober.ResolveManagedTikaBaseURL(ctx)
		if ok, _ := s.prober.ProbeTika(ctx, baseURL, ""); ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("tika_start_timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func (s *Service) waitForManagedRapidOCRReachable(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		baseURL := s.prober.ResolveManagedRapidOCRBaseURL(ctx)
		if ok, _ := s.prober.ProbeRapidOCR(ctx, baseURL, ""); ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("rapidocr_start_timeout")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}
