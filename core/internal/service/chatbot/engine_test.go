package chatbot

import "testing"

func menuGraph() Graph {
	return Graph{SchemaVersion: 1,
		Nodes: []Node{
			{ID: "trigger", Type: "trigger"},
			{ID: "welcome", Type: "message", Data: NodeData{Text: "Welcome"}},
			{ID: "menu", Type: "menu", Data: NodeData{Text: "Choose", Options: []MenuOption{{ID: "products", Label: "Products", Value: "products"}, {ID: "support", Label: "Support", Value: "support"}}}},
			{ID: "products-reply", Type: "message", Data: NodeData{Text: "Our products"}}, {ID: "products-end", Type: "end"},
			{ID: "support-reply", Type: "message", Data: NodeData{Text: "Connecting support"}}, {ID: "support-end", Type: "end"},
		},
		Edges: []Edge{
			{ID: "a", Source: "trigger", Target: "welcome"}, {ID: "b", Source: "welcome", Target: "menu"},
			{ID: "c", Source: "menu", SourceHandle: "products", Target: "products-reply"}, {ID: "d", Source: "products-reply", Target: "products-end"},
			{ID: "e", Source: "menu", SourceHandle: "support", Target: "support-reply"}, {ID: "f", Source: "support-reply", Target: "support-end"},
		}}
}

func TestExecuteStartsAndWaitsAtMenu(t *testing.T) {
	result, err := Execute(menuGraph(), "", "hi")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Waiting || result.CurrentNode != "menu" || len(result.Messages) != 2 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestExecuteResumesNumericAndTextSelections(t *testing.T) {
	for _, input := range []string{"1", "products", "Products"} {
		result, err := Execute(menuGraph(), "menu", input)
		if err != nil {
			t.Fatal(err)
		}
		if !result.Completed || len(result.Messages) != 1 || result.Messages[0] != "Our products" {
			t.Fatalf("input %q: %#v", input, result)
		}
	}
}

func TestExecuteRepeatsMenuForInvalidSelection(t *testing.T) {
	result, err := Execute(menuGraph(), "menu", "99")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Waiting || result.CurrentNode != "menu" || len(result.Messages) != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
}
