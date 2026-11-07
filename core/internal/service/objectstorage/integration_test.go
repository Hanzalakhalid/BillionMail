package objectstorage

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestS3UploadPolicyContract(t *testing.T) {
	endpoint := os.Getenv("BM_TEST_MEDIA_ENDPOINT")
	if endpoint == "" {
		t.Skip("BM_TEST_MEDIA_ENDPOINT is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	store, err := New(ctx, Config{Bucket: "marketing-assets", Region: "us-east-1", Endpoint: endpoint, PublicEndpoint: endpoint, AccessKey: os.Getenv("BM_TEST_MEDIA_ACCESS_KEY"), SecretKey: os.Getenv("BM_TEST_MEDIA_SECRET_KEY"), PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Health(ctx); err != nil {
		t.Fatal(err)
	}
	prefix := "companies/" + uuid.NewString() + "/quarantine/"
	post := func(form UploadForm, data []byte) int {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for key, value := range form.Fields {
			if err := writer.WriteField(key, value); err != nil {
				t.Fatal(err)
			}
		}
		file, err := writer.CreateFormFile("file", "reference.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = file.Write(data); err != nil {
			t.Fatal(err)
		}
		writer.Close()
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, form.URL, &body)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", writer.FormDataContentType())
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		io.Copy(io.Discard, io.LimitReader(response.Body, 8192))
		return response.StatusCode
	}
	key := prefix + "valid"
	form, err := store.PresignUpload(ctx, key, "image/png", 4, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if status := post(form, []byte("data")); status < 200 || status >= 300 {
		t.Fatalf("valid POST status=%d", status)
	}
	body, mime, err := store.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(body)
	body.Close()
	if err != nil || string(data) != "data" || mime != "image/png" {
		t.Fatalf("stored content=%q type=%s err=%v", data, mime, err)
	}
	signed, err := store.PresignGet(ctx, key, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, signed, nil)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("signed download status=%d", response.StatusCode)
	}
	oversized, err := store.PresignUpload(ctx, prefix+"too-large", "image/png", 4, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if status := post(oversized, []byte("oversized")); status >= 200 && status < 300 {
		t.Fatal("storage accepted a file exceeding the signed policy")
	}
	wrongType, err := store.PresignUpload(ctx, prefix+"wrong-type", "image/png", 4, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	wrongType.Fields["Content-Type"] = "image/jpeg"
	if status := post(wrongType, []byte("data")); status >= 200 && status < 300 {
		t.Fatal("storage accepted an altered content type")
	}
	if err = store.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.Get(ctx, key); err == nil {
		t.Fatal("deleted object remained readable")
	}
}
