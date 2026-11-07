package business

import "testing"

func TestProfileValidation(t *testing.T) {
	valid := Input{Name: "Store", Website: "https://example.test/products", Industry: "Retail", Description: "Product business", BrandVoice: "Clear and friendly", Locale: "en", Timezone: "Asia/Karachi"}
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Input){
		func(v *Input) { v.Name = "" }, func(v *Input) { v.Website = "javascript:alert(1)" }, func(v *Input) { v.Website = "https://user:password@example.test" }, func(v *Input) { v.Timezone = "Mars/Olympus" }, func(v *Input) { v.Locale = "" },
	} {
		input := valid
		change(&input)
		if Validate(input) == nil {
			t.Errorf("invalid profile accepted: %+v", input)
		}
	}
}
