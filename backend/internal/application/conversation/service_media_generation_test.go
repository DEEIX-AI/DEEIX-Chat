package conversation

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	appupload "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/upload"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	infrallm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/objectstorage"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestDetectGeneratedImageMIMERejectsNonImageBytes(t *testing.T) {
	_, _, err := validateGeneratedImageBytes([]byte("<html>not an image</html>"), "image/png")
	if err == nil {
		t.Fatal("expected non-image generated output to be rejected")
	}
}

func TestDetectGeneratedImageMIMEUsesActualImageBytes(t *testing.T) {
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00}
	got, mimeType, err := validateGeneratedImageBytes(data, "image/png")
	if err != nil {
		t.Fatalf("expected jpeg bytes to pass validation: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatal("expected validation to return original bytes")
	}
	if mimeType != "image/jpeg" {
		t.Fatalf("expected actual jpeg MIME, got %q", mimeType)
	}
}

func TestNormalizeMediaImageEditInputConvertsCMYKJPEGToPNG(t *testing.T) {
	src := image.NewCMYK(image.Rect(0, 0, 2, 2))
	src.SetCMYK(0, 0, color.CMYK{C: 255, M: 0, Y: 0, K: 0})
	src.SetCMYK(1, 0, color.CMYK{C: 0, M: 255, Y: 0, K: 0})
	src.SetCMYK(0, 1, color.CMYK{C: 0, M: 0, Y: 255, K: 0})
	src.SetCMYK(1, 1, color.CMYK{C: 0, M: 0, Y: 0, K: 32})

	var input bytes.Buffer
	if err := jpeg.Encode(&input, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode source jpeg: %v", err)
	}

	got, mimeType, err := normalizeMediaImageEditInput(input.Bytes(), "image/jpeg")
	if err != nil {
		t.Fatalf("normalize image edit input: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("expected normalized PNG MIME, got %q", mimeType)
	}
	if !bytes.HasPrefix(got, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		prefixLen := len(got)
		if prefixLen > 8 {
			prefixLen = 8
		}
		t.Fatalf("expected normalized PNG bytes, got prefix % x", got[:prefixLen])
	}
	decoded, err := png.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatalf("decode normalized PNG: %v", err)
	}
	if decoded.Bounds().Dx() != 2 || decoded.Bounds().Dy() != 2 {
		t.Fatalf("expected dimensions to be preserved, got %v", decoded.Bounds())
	}
}

func TestMediaImageEditInputFileNameMatchesNormalizedMIME(t *testing.T) {
	got := mediaImageEditInputFileName("IMG_4442.jpeg", "image/png")
	if got != "IMG_4442.png" {
		t.Fatalf("expected normalized filename extension, got %q", got)
	}
}

func TestStripBase64DataURLPrefix(t *testing.T) {
	got := stripBase64DataURLPrefix("data:image/png;base64, aGVsbG8= ")
	if got != "aGVsbG8=" {
		t.Fatalf("unexpected stripped data URL: %q", got)
	}
}

func TestMediaImageStreamEnabledUsesModelCapabilitiesOverride(t *testing.T) {
	tests := []struct {
		name             string
		protocol         string
		upstreamModel    string
		capabilitiesJSON string
		want             bool
	}{
		{
			name:          "openai gpt image defaults to stream",
			protocol:      llm.AdapterOpenAIImageGenerations,
			upstreamModel: "gpt-image-2",
			want:          true,
		},
		{
			name:             "explicit false disables stream",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":false}}`,
			want:             false,
		},
		{
			name:             "explicit true preserves protocol default",
			protocol:         llm.AdapterOpenAIImageGenerations,
			upstreamModel:    "gpt-image-2",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             true,
		},
		{
			name:             "invalid json keeps protocol default",
			protocol:         llm.AdapterGoogleImageGeneration,
			upstreamModel:    "gemini-3-pro-image",
			capabilitiesJSON: `{`,
			want:             true,
		},
		{
			name:          "gemini interactions streams image media",
			protocol:      llm.AdapterGeminiInteractions,
			upstreamModel: "gemini-3.5-flash",
			want:          true,
		},
		{
			name:             "unsupported protocol cannot be enabled by capabilities",
			protocol:         llm.AdapterXAIImage,
			upstreamModel:    "grok-2-image",
			capabilitiesJSON: `{"image":{"stream":true}}`,
			want:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mediaImageStreamEnabled(tt.protocol, tt.upstreamModel, tt.capabilitiesJSON)
			if got != tt.want {
				t.Fatalf("mediaImageStreamEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mediaImageResultRepository struct {
	*conversationRunClaimRepositoryStub
	source      model.FileObject
	attachments []model.Attachment
}

func (r *mediaImageResultRepository) UpdateConversationModel(_ context.Context, _ uint, name string, provider string) error {
	r.conversation.Model = name
	r.conversation.Provider = provider
	return nil
}

func (r *mediaImageResultRepository) CompleteAssistantMessageWithAttachments(_ context.Context, _ uint, _ repository.MessageUsageUpdate, _ uint, _ repository.AssistantMessageCompletionUpdate, attachments []model.Attachment) error {
	r.attachments = attachments
	return nil
}

func (*mediaImageResultRepository) UpdateMessageState(context.Context, uint, string, string, string) error {
	return nil
}

func (*mediaImageResultRepository) GetUserSettingValue(context.Context, uint, string) (string, error) {
	return "", nil
}

func (r *mediaImageResultRepository) GetActiveFileObjectsByIDs(context.Context, uint, []string) ([]model.FileObject, error) {
	return []model.FileObject{r.source}, nil
}

type mediaImageResultUploadRepository struct {
	generatedImageUploadRepository
	source model.FileObject
}

func (r mediaImageResultUploadRepository) GetActiveFileObjectByID(context.Context, uint, string) (*model.FileObject, error) {
	return &r.source, nil
}

func (mediaImageResultUploadRepository) TouchFileObjectLastAccessedAt(context.Context, uint, string, time.Time) error {
	return nil
}

// 上游在同一条目里同时返回 b64_json 与 url 时，图片任务直接保存内联字节，不下载 url：
// 下载可能超时或被重置，不能让已经拿到结果的生成因此失败。生成与编辑、JSON 与 SSE 都覆盖。
func TestStreamMediaImageSavesInlineBase64WithoutDownloading(t *testing.T) {
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(encoded.Bytes())
	for _, task := range []MediaImageTaskType{MediaImageTaskGeneration, MediaImageTaskEdit} {
		for _, format := range []string{"json", "sse", "stream json"} {
			t.Run(string(task)+"/"+format, func(t *testing.T) {
				var responseEntry map[string]string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if task == MediaImageTaskEdit && r.URL.Path != "/v1/images/edits" {
						t.Error("edit task used the wrong endpoint")
					}
					body, _ := json.Marshal(map[string]any{"data": []any{responseEntry}})
					if format == "sse" {
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = fmt.Fprintf(w, "event: image_generation.completed\ndata: %s\n\n", body)
					} else {
						w.Header().Set("Content-Type", "application/json")
						_, _ = w.Write(body)
					}
				}))
				defer server.Close()
				store := objectstorage.NewLocal(t.TempDir())
				if _, err := store.Put(t.Context(), "source.png", bytes.NewReader(encoded.Bytes()), objectstorage.PutOptions{ContentType: "image/png"}); err != nil {
					t.Fatal(err)
				}
				source := model.FileObject{FileID: "source", UserID: 1, FileName: "source.png", MimeType: "image/png", DetectedMIME: "image/png", StoragePath: "source.png", SizeBytes: int64(encoded.Len())}
				repo := &mediaImageResultRepository{
					conversationRunClaimRepositoryStub: &conversationRunClaimRepositoryStub{
						runs: make(map[string]model.Run), conversation: model.Conversation{ID: 11, UserID: 1, PublicID: "conversation-11"},
					},
					source: source,
				}
				runtimeCfg := config.NewRuntime(config.Config{})
				uploadSvc := appupload.NewServiceWithRuntime(runtimeCfg, mediaImageResultUploadRepository{source: source}, nil, appupload.Hooks{}, appupload.ErrorSet{}, "test")
				uploadSvc.SetObjectStoreProvider(&conversationTestStoreProvider{store: store})
				client := infrallm.NewClient(security.OutboundPolicy{})
				defer client.CloseIdleConnections()
				protocol := llm.AdapterOpenAIImageGenerations
				if task == MediaImageTaskEdit {
					protocol = llm.AdapterOpenAIImageEdits
				}
				resolver := &textTaskRouteResolverStub{routes: make(map[string]*channel.ResolvedRoute)}
				registry := newGenerationStreamRegistry(nil, generationStreamOptions{})
				defer registry.close()
				service := &Service{cfg: runtimeCfg, repo: repo, uploadSvc: uploadSvc, llmClient: client, routeResolver: resolver, generationStreams: registry}
				// 非流式用 image.stream=false 关掉流式；其余两种按协议默认走流式。
				caps := `{}`
				if format == "json" {
					caps = `{"image":{"stream":false}}`
				}
				for index, test := range []struct {
					name          string
					entry         map[string]string
					downloadFails bool
					wantDownloads int
					wantStage     string
				}{
					{"both", map[string]string{"b64_json": b64, "url": "https://images.example.test/unavailable"}, true, 0, ""},
					{"url only", map[string]string{"url": "https://images.example.test/result.png"}, false, 1, ""},
					{"base64 only", map[string]string{"b64_json": b64}, true, 0, ""},
					{"blank base64", map[string]string{"b64_json": " \t", "url": "https://images.example.test/result.png"}, false, 1, ""},
					// 非空但无效的字节明确报错，不悄悄改为下载 url。
					{"invalid base64", map[string]string{"b64_json": "invalid!", "url": "https://images.example.test/result.png"}, false, 0, "decode"},
					{"not an image", map[string]string{"b64_json": base64.StdEncoding.EncodeToString([]byte("not an image")), "url": "https://images.example.test/result.png"}, false, 0, "validation"},
				} {
					t.Run(test.name, func(t *testing.T) {
						responseEntry = test.entry
						name := fmt.Sprintf("image-model-%d", index)
						resolver.routes[name] = &channel.ResolvedRoute{PlatformModelName: name, UpstreamID: uint(index + 1), Protocol: protocol, BaseURL: server.URL, UpstreamModel: "gpt-image-1", ModelCapabilitiesJSON: caps}
						downloads := 0
						service.mediaDownloader = generatedMediaDownloaderStub{downloadImage: func(context.Context, string, string, int64) ([]byte, string, error) {
							downloads++
							if test.downloadFails {
								return nil, "", errors.New("download unavailable")
							}
							return encoded.Bytes(), "image/png", nil
						}}
						repo.attachments = nil
						input := MediaImageInput{UserID: 1, ConversationID: 11, TaskType: task, PlatformModelName: name, Prompt: "draw", ClientRunID: fmt.Sprintf("run-%d", index)}
						if task == MediaImageTaskEdit {
							input.FileIDs = []string{"source"}
						}
						ctx, release, ok := registry.acquireRun(t.Context())
						if !ok {
							t.Fatal("acquire generation lifecycle")
						}
						defer release()
						defer registry.finish(ctx, input.ClientRunID)
						result, err := service.StreamMediaImage(ctx, input)
						if downloads != test.wantDownloads {
							t.Fatalf("downloads = %d, want %d", downloads, test.wantDownloads)
						}
						if test.wantStage != "" {
							if !errors.Is(err, ErrGeneratedMediaArtifactUnavailable) || generatedMediaArtifactFailureDetails(err).stage != test.wantStage {
								t.Fatalf("unexpected failure stage: %v", err)
							}
							if len(repo.attachments) != 0 {
								t.Fatal("invalid result was saved")
							}
							return
						}
						if err != nil || result == nil || result.AssistantMessage.Status != "success" || len(repo.attachments) != 1 {
							t.Fatalf("image task did not save successfully: %v", err)
						}
						reader, _, err := store.Open(t.Context(), repo.attachments[0].StoragePath)
						if err != nil {
							t.Fatal(err)
						}
						defer reader.Close() //nolint:errcheck
						data, err := io.ReadAll(reader)
						if err != nil || !bytes.Equal(data, encoded.Bytes()) || repo.attachments[0].MimeType != "image/png" {
							t.Fatal("saved image bytes or MIME did not match")
						}
					})
				}
			})
		}
	}
}
