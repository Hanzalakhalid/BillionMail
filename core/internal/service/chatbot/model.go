package chatbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const SchemaVersion = 1

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type MenuOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type NodeData struct {
	Text    string       `json:"text,omitempty"`
	Options []MenuOption `json:"options,omitempty"`
}

type Node struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Position Position `json:"position"`
	Data     NodeData `json:"data"`
}

type Edge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	Target       string `json:"target"`
	SourceHandle string `json:"source_handle,omitempty"`
}

type Graph struct {
	SchemaVersion int    `json:"schema_version"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}

type Flow struct {
	ID                 uuid.UUID `json:"id"`
	CompanyID          uuid.UUID `json:"company_id"`
	PhoneNumberID      uuid.UUID `json:"phone_number_id"`
	DisplayPhoneNumber string    `json:"display_phone_number"`
	VerifiedName       string    `json:"verified_name"`
	Name               string    `json:"name"`
	Status             string    `json:"status"`
	DraftVersion       int       `json:"draft_version"`
	PublishedVersion   int       `json:"published_version"`
	Graph              Graph     `json:"graph"`
	CreatedAt          string    `json:"created_at"`
	UpdatedAt          string    `json:"updated_at"`
}

func ValidateGraph(graph Graph) error {
	if graph.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported graph schema version %d", graph.SchemaVersion)
	}
	if len(graph.Nodes) < 3 || len(graph.Nodes) > 100 {
		return errors.New("a bot needs 3 to 100 nodes")
	}
	if len(graph.Edges) == 0 || len(graph.Edges) > 200 {
		return errors.New("a bot needs 1 to 200 connections")
	}
	nodes := make(map[string]Node, len(graph.Nodes))
	triggers := 0
	for _, node := range graph.Nodes {
		node.ID = strings.TrimSpace(node.ID)
		if node.ID == "" || len(node.ID) > 100 {
			return errors.New("every node needs a valid id")
		}
		if _, exists := nodes[node.ID]; exists {
			return fmt.Errorf("duplicate node id %q", node.ID)
		}
		switch node.Type {
		case "trigger":
			triggers++
		case "message":
			if strings.TrimSpace(node.Data.Text) == "" || len(node.Data.Text) > 4096 {
				return fmt.Errorf("message node %s needs text up to 4096 characters", node.ID)
			}
		case "menu":
			if strings.TrimSpace(node.Data.Text) == "" {
				return fmt.Errorf("menu node %s needs prompt text", node.ID)
			}
			if len(node.Data.Options) < 1 || len(node.Data.Options) > 10 {
				return fmt.Errorf("menu node %s needs 1 to 10 options", node.ID)
			}
			seen := map[string]bool{}
			for _, option := range node.Data.Options {
				if strings.TrimSpace(option.ID) == "" || strings.TrimSpace(option.Label) == "" || strings.TrimSpace(option.Value) == "" {
					return fmt.Errorf("menu node %s has an incomplete option", node.ID)
				}
				if seen[option.ID] || seen[strings.ToLower(option.Value)] {
					return fmt.Errorf("menu node %s has duplicate options", node.ID)
				}
				seen[option.ID], seen[strings.ToLower(option.Value)] = true, true
			}
		case "end":
		default:
			return fmt.Errorf("unsupported node type %q", node.Type)
		}
		nodes[node.ID] = node
	}
	if triggers != 1 {
		return errors.New("a bot must have exactly one trigger")
	}
	outgoing := map[string][]Edge{}
	for _, edge := range graph.Edges {
		if _, ok := nodes[edge.Source]; !ok {
			return fmt.Errorf("connection %s has an unknown source", edge.ID)
		}
		if _, ok := nodes[edge.Target]; !ok {
			return fmt.Errorf("connection %s has an unknown target", edge.ID)
		}
		outgoing[edge.Source] = append(outgoing[edge.Source], edge)
	}
	for _, node := range graph.Nodes {
		if node.Type == "end" {
			continue
		}
		if node.Type == "menu" {
			for _, option := range node.Data.Options {
				found := false
				for _, edge := range outgoing[node.ID] {
					if edge.SourceHandle == option.ID {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("menu option %q is not connected", option.Label)
				}
			}
		} else if len(outgoing[node.ID]) != 1 {
			return fmt.Errorf("%s node %s must have exactly one outgoing connection", node.Type, node.ID)
		}
	}
	start := ""
	for _, node := range graph.Nodes {
		if node.Type == "trigger" {
			start = node.ID
		}
	}
	reachable := map[string]bool{}
	var visit func(string)
	visit = func(id string) {
		if reachable[id] {
			return
		}
		reachable[id] = true
		for _, e := range outgoing[id] {
			visit(e.Target)
		}
	}
	visit(start)
	if len(reachable) != len(nodes) {
		return errors.New("every node must be reachable from the trigger")
	}
	return nil
}

func DecodeGraph(value any) (Graph, error) {
	var raw []byte
	switch typed := value.(type) {
	case []byte:
		raw = typed
	case string:
		raw = []byte(typed)
	default:
		raw, _ = json.Marshal(value)
	}
	var graph Graph
	err := json.Unmarshal(raw, &graph)
	return graph, err
}
