package pdfrender

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
)

const (
	errRendererUnavailable = "pdf_page_renderer_unavailable"
	errRenderUnsupported   = "pdf_page_rendering_unsupported"
	errPageExtractFailed   = "pdf_page_extract_failed"
	errPageRenderFailed    = "pdf_page_render_failed"
)

type Request struct {
	SourcePath string
	PageNumber int
	TempDir    string
}

type Renderer struct {
	backend renderBackend
}

type renderBackend interface {
	Name() string
	RenderPageJPEG(ctx context.Context, singlePagePDFPath string, outputBasePath string) (string, error)
}

type commandBackend struct {
	name       string
	binary     string
	args       func(singlePagePDFPath string, outputBasePath string) []string
	outputPath func(outputBasePath string) string
}

func New() *Renderer {
	return &Renderer{
		backend: detectBackend(),
	}
}

func (r *Renderer) RenderPageJPEG(ctx context.Context, req Request) ([]byte, error) {
	if r == nil || r.backend == nil {
		return nil, fmt.Errorf(errRendererUnavailable)
	}
	sourcePath := strings.TrimSpace(req.SourcePath)
	if sourcePath == "" {
		return nil, fmt.Errorf("pdf_invalid_path")
	}
	if req.PageNumber <= 0 {
		return nil, fmt.Errorf("invalid_page_number")
	}

	tempDir := strings.TrimSpace(req.TempDir)
	ownedTempDir := false
	if tempDir == "" {
		createdDir, err := os.MkdirTemp("", "deeix-chat-pdf-render-*")
		if err != nil {
			return nil, err
		}
		tempDir = createdDir
		ownedTempDir = true
	}
	if ownedTempDir {
		defer os.RemoveAll(tempDir)
	}

	singlePagePDFPath := filepath.Join(tempDir, fmt.Sprintf("page-%d.pdf", req.PageNumber))
	if err := writeSinglePagePDF(sourcePath, singlePagePDFPath, req.PageNumber); err != nil {
		return nil, err
	}

	outputBasePath := filepath.Join(tempDir, fmt.Sprintf("page-%d", req.PageNumber))
	renderedPath, err := r.backend.RenderPageJPEG(ctx, singlePagePDFPath, outputBasePath)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(renderedPath)
}

func detectBackend() renderBackend {
	for _, candidate := range backendCandidatesForOS(runtime.GOOS) {
		if _, err := exec.LookPath(candidate.binary); err == nil {
			return candidate
		}
	}
	return nil
}

func backendCandidatesForOS(goos string) []commandBackend {
	pdftoppm := commandBackend{
		name:   "pdftoppm",
		binary: "pdftoppm",
		args: func(singlePagePDFPath string, outputBasePath string) []string {
			return []string{"-jpeg", "-singlefile", singlePagePDFPath, outputBasePath}
		},
		outputPath: func(outputBasePath string) string {
			return outputBasePath + ".jpg"
		},
	}
	mutool := commandBackend{
		name:   "mutool",
		binary: "mutool",
		args: func(singlePagePDFPath string, outputBasePath string) []string {
			return []string{"draw", "-q", "-F", "jpg", "-o", outputBasePath + ".jpg", singlePagePDFPath, "1"}
		},
		outputPath: func(outputBasePath string) string {
			return outputBasePath + ".jpg"
		},
	}
	magick := commandBackend{
		name:   "magick",
		binary: "magick",
		args: func(singlePagePDFPath string, outputBasePath string) []string {
			return []string{"-density", "144", singlePagePDFPath + "[0]", "-quality", "85", outputBasePath + ".jpg"}
		},
		outputPath: func(outputBasePath string) string {
			return outputBasePath + ".jpg"
		},
	}
	sips := commandBackend{
		name:   "sips",
		binary: "sips",
		args: func(singlePagePDFPath string, outputBasePath string) []string {
			return []string{"-s", "format", "jpeg", singlePagePDFPath, "--out", outputBasePath + ".jpg"}
		},
		outputPath: func(outputBasePath string) string {
			return outputBasePath + ".jpg"
		},
	}

	switch goos {
	case "darwin":
		return []commandBackend{sips, pdftoppm, mutool, magick}
	case "linux":
		return []commandBackend{pdftoppm, mutool, magick}
	case "windows":
		return []commandBackend{pdftoppm, mutool, magick}
	default:
		return nil
	}
}

func (b commandBackend) Name() string {
	return b.name
}

func (b commandBackend) RenderPageJPEG(ctx context.Context, singlePagePDFPath string, outputBasePath string) (string, error) {
	command := exec.CommandContext(ctx, b.binary, b.args(singlePagePDFPath, outputBasePath)...)
	output, err := command.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return "", fmt.Errorf("%s: %s", errPageRenderFailed, b.name)
		}
		return "", fmt.Errorf("%s: %s: %s", errPageRenderFailed, b.name, detail)
	}
	return b.outputPath(outputBasePath), nil
}

func writeSinglePagePDF(sourcePath string, outputPath string, pageNumber int) error {
	sourceFile, err := os.Open(strings.TrimSpace(sourcePath))
	if err != nil {
		return err
	}
	defer sourceFile.Close() //nolint:errcheck

	outputFile, err := os.Create(strings.TrimSpace(outputPath))
	if err != nil {
		return err
	}
	defer outputFile.Close() //nolint:errcheck

	if err := pdfcpuapi.Trim(sourceFile, outputFile, []string{strconv.Itoa(pageNumber)}, nil); err != nil {
		return fmt.Errorf("%s: %w", errPageExtractFailed, err)
	}
	return nil
}
