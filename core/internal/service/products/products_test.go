package products

import (
	"github.com/google/uuid"
	"testing"
)

func TestValidateProduct(t *testing.T) {
	good := Input{Name: "Reference product", SKU: "SKU-1", Description: "A reusable product", Benefits: []string{"Reusable"}, URL: "https://example.test/product", Price: "12.99", Currency: "USD", AssetVersionIDs: []uuid.UUID{uuid.New()}}
	if err := Validate(good); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Input){
		func(v *Input) { v.Name = "" }, func(v *Input) { v.Price = "-1" }, func(v *Input) { v.Price = "1e4" }, func(v *Input) { v.Currency = "" }, func(v *Input) { v.URL = "file:///etc/passwd" }, func(v *Input) { v.URL = "https://user:pass@example.test" }, func(v *Input) { v.AssetVersionIDs = []uuid.UUID{uuid.Nil} }, func(v *Input) { v.AssetVersionIDs = []uuid.UUID{good.AssetVersionIDs[0], good.AssetVersionIDs[0]} },
	} {
		bad := good
		change(&bad)
		if Validate(bad) == nil {
			t.Errorf("invalid product accepted: %+v", bad)
		}
	}
}

func TestProductCursor(t *testing.T) {
	if _, _, err := decodeCursor("invalid"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}
