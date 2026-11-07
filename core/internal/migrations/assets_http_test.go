package migrations

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	assetscontroller "billionmail-core/internal/controller/assets"
	businesscontroller "billionmail-core/internal/controller/business"
	productscontroller "billionmail-core/internal/controller/products"
	projectscontroller "billionmail-core/internal/controller/projects"
	"billionmail-core/internal/service/assets"
	"billionmail-core/internal/service/business"
	"billionmail-core/internal/service/middlewares"
	"billionmail-core/internal/service/objectstorage"
	"billionmail-core/internal/service/products"
	"billionmail-core/internal/service/projects"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/tenancy"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
)

// This contract uses the real GoFrame binder/response middleware and MinIO.
// Authentication itself is covered separately; a fixture membership is injected.
func testAssetHTTPContract(t *testing.T, ctx context.Context, tenant tenancy.TenantContext) {
	endpoint := os.Getenv("BM_TEST_MEDIA_ENDPOINT")
	if endpoint == "" {
		t.Skip("BM_TEST_MEDIA_ENDPOINT is not configured")
	}
	for name, value := range map[string]string{"MEDIA_BUCKET": "marketing-assets", "MEDIA_REGION": "us-east-1", "MEDIA_ENDPOINT": endpoint, "MEDIA_PUBLIC_ENDPOINT": endpoint, "MEDIA_ACCESS_KEY": os.Getenv("BM_TEST_MEDIA_ACCESS_KEY"), "MEDIA_SECRET_KEY": os.Getenv("BM_TEST_MEDIA_SECRET_KEY"), "MEDIA_PATH_STYLE": "true"} {
		t.Setenv(name, value)
	}
	store, err := objectstorage.New(ctx, objectstorage.ConfigFromEnv())
	if err != nil {
		t.Fatal(err)
	}
	server := g.Server("asset-http-" + uuid.NewString())
	server.SetAddr("127.0.0.1:0")
	server.SetDumpRouterMap(false)
	server.SetLogStdout(false)
	server.SetAccessLogEnabled(false)
	server.Group("/api", func(group *ghttp.RouterGroup) {
		group.Middleware(middlewares.HandleApiResponse, func(request *ghttp.Request) {
			request.SetCtx(tenancy.WithContext(request.Context(), tenant))
			request.Middleware.Next()
		})
		group.Bind(assetscontroller.NewV1())
		group.Bind(businesscontroller.NewV1())
		group.Bind(productscontroller.NewV1())
		group.Bind(projectscontroller.NewV1())
	})
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer server.Shutdown()
	base := fmt.Sprintf("http://127.0.0.1:%d/api", server.GetListenedPort())
	client := &http.Client{Timeout: 20 * time.Second}
	send := func(path string, body any, key string) (int, []byte) {
		t.Helper()
		encoded, _ := json.Marshal(body)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, base+path, bytes.NewReader(encoded))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", key)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		result, err := io.ReadAll(io.LimitReader(response.Body, 64*1024))
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, result
	}
	var imageData bytes.Buffer
	if err := png.Encode(&imageData, image.NewRGBA(image.Rect(0, 0, 6, 4))); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	status, data := send("/v1/assets/uploads", assets.UploadInput{Title: "HTTP reference", ContentType: "image/png", ByteSize: int64(imageData.Len())}, key)
	var upload assets.Upload
	if status != 200 || json.Unmarshal(data, &upload) != nil || upload.AssetID == uuid.Nil || upload.Form == nil {
		t.Fatalf("direct upload response status=%d body=%s", status, data)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range upload.Form.Fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatal(err)
		}
	}
	file, err := writer.CreateFormFile("file", "reference.png")
	if err != nil {
		t.Fatal(err)
	}
	file.Write(imageData.Bytes())
	writer.Close()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, upload.Form.URL, &body)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("storage upload status=%d", response.StatusCode)
	}
	status, data = send("/v1/assets/uploads/complete", g.Map{"version_id": upload.VersionID}, key+":complete")
	var job assets.Job
	if status != http.StatusAccepted || json.Unmarshal(data, &job) != nil || job.JobID == uuid.Nil || job.Status != "queued" || job.ResourceStatus != "processing" {
		t.Fatalf("async upload response status=%d body=%s", status, data)
	}
	var event platformqueue.Event
	if err := g.DB().Model("outbox_events").Where("company_id", tenant.CompanyID).Where("id", job.JobID).Scan(&event); err != nil {
		t.Fatal(err)
	}
	if err := (assets.Validator{Store: store, Scanner: assetScanner{}}).Handle(ctx, event); err != nil {
		t.Fatal(err)
	}
	response, err = client.Get(base + "/v1/assets/download?version_id=" + upload.VersionID.String())
	if err != nil {
		t.Fatal(err)
	}
	var download struct {
		URL string `json:"url"`
	}
	err = json.NewDecoder(response.Body).Decode(&download)
	response.Body.Close()
	if err != nil || download.URL == "" {
		t.Fatalf("download response=%+v err=%v", download, err)
	}
	response, err = client.Get(download.URL)
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(response.Body)
	response.Body.Close()
	if err != nil || !bytes.Equal(data, imageData.Bytes()) {
		t.Fatalf("validated bytes did not survive storage handoff: %v", err)
	}
	response, err = client.Get(base + "/v1/assets/list?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	var page assets.Page
	err = json.NewDecoder(response.Body).Decode(&page)
	response.Body.Close()
	if err != nil || len(page.Items) == 0 || len(page.Items) > 2 {
		t.Fatalf("direct page response=%+v err=%v", page, err)
	}
	t.Run("project discovery and reconnect", func(t *testing.T) {
		input := projects.Input{Title: "HTTP discovery", Objective: "Launch a product", Requirements: projects.Requirements{Channels: []string{"email", "sms"}, Languages: []string{"en"}}}
		status, data := send("/v1/projects/save", input, uuid.NewString())
		var project projects.Project
		if status != 200 || json.Unmarshal(data, &project) != nil || project.ID == "" || len(project.Missing) != 2 {
			t.Fatalf("project status=%d body=%s", status, data)
		}
		status, data = send("/v1/projects/messages", g.Map{"id": project.ID, "content": "Use a warm voice"}, uuid.NewString())
		if status != 200 {
			t.Fatalf("message status=%d body=%s", status, data)
		}
		response, err := client.Get(base + "/v1/projects/events?id=" + project.ID)
		if err != nil {
			t.Fatal(err)
		}
		var page projects.EventsPage
		err = json.NewDecoder(response.Body).Decode(&page)
		response.Body.Close()
		if err != nil || len(page.Items) != 2 {
			t.Fatalf("project events=%+v err=%v", page, err)
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/projects/stream?id="+project.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Last-Event-ID", fmt.Sprint(page.Items[0].ID))
		response, err = client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		if response.Header.Get("Content-Type") != "text/event-stream; charset=utf-8" {
			t.Fatalf("stream headers=%v", response.Header)
		}
		reader := bufio.NewScanner(response.Body)
		foundID, foundType, foundData := false, false, false
		for reader.Scan() {
			line := reader.Text()
			if line == fmt.Sprintf("id: %d", page.Items[1].ID) {
				foundID = true
			}
			if line == "event: message.created" {
				foundType = true
			}
			if len(line) > 6 && line[:6] == "data: " {
				foundData = true
				break
			}
		}
		response.Body.Close()
		if !foundID || !foundType || !foundData {
			t.Fatalf("SSE reconnect id=%v type=%v data=%v", foundID, foundType, foundData)
		}
		request, err = http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/projects/stream?id="+project.ID, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Last-Event-ID", "bad")
		response, err = client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != 400 {
			t.Fatalf("bad SSE cursor status=%d", response.StatusCode)
		}
	})
	t.Run("product HTTP versions", func(t *testing.T) {
		input := products.Input{Name: "HTTP product", Price: "19.95", Currency: "USD", AssetVersionIDs: []uuid.UUID{upload.VersionID}}
		key := uuid.NewString()
		status, data := send("/v1/products/save", input, key)
		var first products.Product
		if status != 200 || json.Unmarshal(data, &first) != nil || first.ID == uuid.Nil || first.VersionID == uuid.Nil || first.Revision != 1 {
			t.Fatalf("product response status=%d body=%s", status, data)
		}
		status, data = send("/v1/products/save", input, key)
		var duplicate products.Product
		json.Unmarshal(data, &duplicate)
		if status != 200 || duplicate.VersionID != first.VersionID {
			t.Fatalf("product duplicate status=%d body=%s", status, data)
		}
		input.ID = first.ID
		input.Revision = 1
		input.Description = "Updated through HTTP"
		status, data = send("/v1/products/save", input, uuid.NewString())
		if status != 200 {
			t.Fatalf("product update status=%d body=%s", status, data)
		}
		status, _ = send("/v1/products/save", input, uuid.NewString())
		if status != http.StatusConflict {
			t.Fatalf("stale product status=%d", status)
		}
		response, err := client.Get(base + "/v1/products/versions?id=" + first.ID.String())
		if err != nil {
			t.Fatal(err)
		}
		var history struct {
			Items []products.Version `json:"items"`
		}
		err = json.NewDecoder(response.Body).Decode(&history)
		response.Body.Close()
		if err != nil || len(history.Items) != 2 || history.Items[1].ID != first.VersionID {
			t.Fatalf("product history=%+v err=%v", history, err)
		}
		status, data = send("/v1/products/delete", g.Map{"id": first.ID}, "delete:"+first.ID.String())
		if status != 200 {
			t.Fatalf("product deletion status=%d body=%s", status, data)
		}
		response, err = client.Get(base + "/v1/products/get?id=" + first.ID.String())
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("removed product status=%d", response.StatusCode)
		}
	})
	t.Run("business HTTP revisions", func(t *testing.T) {
		response, err := client.Get(base + "/v1/business/profile")
		if err != nil {
			t.Fatal(err)
		}
		var current struct {
			Profile *business.Profile `json:"profile"`
		}
		err = json.NewDecoder(response.Body).Decode(&current)
		response.Body.Close()
		if err != nil || current.Profile == nil {
			t.Fatalf("profile response=%+v err=%v", current, err)
		}
		input := current.Profile.Input
		input.BrandVoice = "Confident and clear"
		key := uuid.NewString()
		for i := 0; i < 2; i++ {
			encoded, _ := json.Marshal(input)
			request, _ := http.NewRequestWithContext(ctx, http.MethodPut, base+"/v1/business/profile", bytes.NewReader(encoded))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Idempotency-Key", key)
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			var saved business.Profile
			err = json.NewDecoder(response.Body).Decode(&saved)
			response.Body.Close()
			if response.StatusCode != 200 || err != nil || saved.Revision != input.Revision+1 {
				t.Fatalf("profile save=%+v status=%d err=%v", saved, response.StatusCode, err)
			}
		}
		encoded, _ := json.Marshal(input)
		request, _ := http.NewRequestWithContext(ctx, http.MethodPut, base+"/v1/business/profile", bytes.NewReader(encoded))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Idempotency-Key", uuid.NewString())
		response, err = client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusConflict {
			t.Fatalf("stale profile status=%d", response.StatusCode)
		}
	})
}
