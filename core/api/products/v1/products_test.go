package v1

import (
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/google/uuid"
	"testing"
)

func TestSaveRequestBinding(t *testing.T) {
	var req SaveReq
	id := uuid.New().String()
	if err := gconv.Struct(map[string]any{"name": "Product", "asset_version_ids": []any{id}}, &req); err != nil {
		t.Fatalf("%+v", err)
	}
	if len(req.AssetVersionIDs) != 1 {
		t.Fatal("missing asset reference")
	}
}
