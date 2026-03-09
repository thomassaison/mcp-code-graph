package graph

import (
	"testing"
)

func TestNodeID(t *testing.T) {
	tests := []struct {
		name     string
		node     *Node
		expected string
	}{
		{
			name: "function node",
			node: &Node{
				Type:    NodeTypeFunction,
				Package: "main",
				Name:    "handleRequest",
				File:    "handler.go",
				Line:    42,
			},
			expected: "func_main_handleRequest_handler.go:42",
		},
		{
			name: "method node",
			node: &Node{
				Type:    NodeTypeMethod,
				Package: "api",
				Name:    "GetUser",
				File:    "user.go",
				Line:    15,
			},
			expected: "func_api_GetUser_user.go:15",
		},
		{
			name: "type node",
			node: &Node{
				Type:    NodeTypeType,
				Package: "models",
				Name:    "User",
			},
			expected: "type_models_User",
		},
		{
			name: "interface node",
			node: &Node{
				Type:    NodeTypeInterface,
				Package: "storage",
				Name:    "Repository",
			},
			expected: "type_storage_Repository",
		},
		{
			name: "package node",
			node: &Node{
				Type: NodeTypePackage,
				Name: "github.com/example/pkg",
			},
			expected: "pkg_github.com/example/pkg",
		},
		{
			name: "file node",
			node: &Node{
				Type: NodeTypeFile,
				Name: "main.go",
			},
			expected: "file_main.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.node.ID(); got != tt.expected {
				t.Errorf("ID() = %q, want %q", got, tt.expected)
			}
		})
	}
}
