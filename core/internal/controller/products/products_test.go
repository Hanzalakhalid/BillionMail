package products

import (
	v1 "billionmail-core/api/products/v1"
	"github.com/google/uuid"
	"testing"
)

func TestSaveInputIDs(t *testing.T) {
	id := uuid.New()
	input, err := saveInput(&v1.SaveReq{ID: id.String(), AssetVersionIDs: []string{id.String()}})
	if err != nil || input.ID != id || len(input.AssetVersionIDs) != 1 || input.AssetVersionIDs[0] != id {
		t.Fatalf("input=%+v err=%v", input, err)
	}
	for _, req := range []*v1.SaveReq{
		{ID: "bad"}, {AssetVersionIDs: []string{"bad"}}, {AssetVersionIDs: []string{uuid.Nil.String()}},
	} {
		if _, err := saveInput(req); err == nil {
			t.Fatal("accepted invalid UUID")
		}
	}
}
