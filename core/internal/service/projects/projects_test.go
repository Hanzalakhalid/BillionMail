package projects

import "testing"

func TestDiscoveryRequirements(t *testing.T) {
	input := Input{Title: "Launch", Objective: "Launch a product"}
	if err := Validate(input); err != nil {
		t.Fatal(err)
	}
	if len(Missing(input.Requirements)) == 0 {
		t.Fatal("empty requirements ready for planning")
	}
	input.Requirements = Requirements{Audience: "Existing subscribers", Channels: []string{"email", "sms"}, Languages: []string{"en", "ur"}, SuccessMetric: "Purchases"}
	if err := Validate(input); err != nil {
		t.Fatal(err)
	}
	if len(Missing(input.Requirements)) != 0 {
		t.Fatal("complete discovery still missing fields")
	}
	input.Requirements.Channels = []string{"email", "email"}
	if Validate(input) == nil {
		t.Fatal("accepted duplicate channels")
	}
	input.Requirements.Channels = []string{"made-up"}
	if Validate(input) == nil {
		t.Fatal("accepted unknown channel")
	}
	input.Requirements.Channels = []string{"email"}
	input.Requirements.Languages = []string{"<script>"}
	if Validate(input) == nil {
		t.Fatal("accepted invalid language")
	}
}

func TestProjectBounds(t *testing.T) {
	for _, input := range []Input{{}, {Title: "Launch"}, {Title: "Launch", Objective: "Sell", Revision: -1}} {
		if Validate(input) == nil {
			t.Fatal("accepted invalid project")
		}
	}
}
