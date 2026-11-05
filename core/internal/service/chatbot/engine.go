package chatbot

import (
	"fmt"
	"strconv"
	"strings"
)

type EngineResult struct {
	Messages    []string
	CurrentNode string
	Waiting     bool
	Completed   bool
}

// Execute advances an immutable graph until it reaches a menu or end node.
// currentNode is the menu node on resume and empty for a new conversation.
func Execute(graph Graph, currentNode, input string) (EngineResult, error) {
	if err := ValidateGraph(graph); err != nil {
		return EngineResult{}, err
	}
	nodes := map[string]Node{}
	edges := map[string][]Edge{}
	start := ""
	for _, n := range graph.Nodes {
		nodes[n.ID] = n
		if n.Type == "trigger" {
			start = n.ID
		}
	}
	for _, e := range graph.Edges {
		edges[e.Source] = append(edges[e.Source], e)
	}
	result := EngineResult{}
	nodeID := start
	if currentNode != "" {
		menu, ok := nodes[currentNode]
		if !ok || menu.Type != "menu" {
			return result, fmt.Errorf("conversation state points to an invalid menu")
		}
		optionID := matchOption(menu.Data.Options, input)
		if optionID == "" {
			result.Messages = append(result.Messages, "Please choose one of the available options.\n\n"+renderMenu(menu))
			result.CurrentNode, result.Waiting = menu.ID, true
			return result, nil
		}
		for _, edge := range edges[menu.ID] {
			if edge.SourceHandle == optionID {
				nodeID = edge.Target
				break
			}
		}
	}
	for steps := 0; steps < 100; steps++ {
		node, ok := nodes[nodeID]
		if !ok {
			return result, fmt.Errorf("node %q was not found", nodeID)
		}
		switch node.Type {
		case "message":
			result.Messages = append(result.Messages, node.Data.Text)
		case "menu":
			result.Messages = append(result.Messages, renderMenu(node))
			result.CurrentNode, result.Waiting = node.ID, true
			return result, nil
		case "end":
			result.Completed = true
			return result, nil
		}
		out := edges[node.ID]
		if len(out) == 0 {
			return result, fmt.Errorf("node %q has no next connection", node.ID)
		}
		nodeID = out[0].Target
	}
	return result, errorsNew("bot exceeded the maximum execution steps")
}

func renderMenu(node Node) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(node.Data.Text))
	for index, option := range node.Data.Options {
		fmt.Fprintf(&b, "\n%d. %s", index+1, strings.TrimSpace(option.Label))
	}
	return b.String()
}

func matchOption(options []MenuOption, input string) string {
	value := strings.TrimSpace(input)
	if number, err := strconv.Atoi(value); err == nil && number >= 1 && number <= len(options) {
		return options[number-1].ID
	}
	for _, option := range options {
		if strings.EqualFold(value, strings.TrimSpace(option.Value)) || strings.EqualFold(value, strings.TrimSpace(option.Label)) {
			return option.ID
		}
	}
	return ""
}

func errorsNew(value string) error { return fmt.Errorf("%s", value) }
