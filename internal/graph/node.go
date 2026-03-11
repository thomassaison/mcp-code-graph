package graph

import "fmt"

type NodeType string

const (
	NodeTypeFunction  NodeType = "function"
	NodeTypeMethod    NodeType = "method"
	NodeTypeType      NodeType = "type"
	NodeTypeInterface NodeType = "interface"
	NodeTypePackage   NodeType = "package"
	NodeTypeFile      NodeType = "file"
)

type Node struct {
	ID        string
	Type      NodeType
	Package   string
	Name      string
	File      string
	Line      int
	Column    int
	Signature string
	Docstring string
	Code      string         `json:"-"` // raw source, not persisted
	Summary   *Summary
	Methods   []Method `json:"methods,omitempty"` // For interfaces: required method signatures
	Metadata  map[string]any
}

type Summary struct {
	Text        string `json:"text"`
	GeneratedBy string `json:"generated_by"`
	Model       string `json:"model"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type Method struct {
	Name      string `json:"name"`
	Signature string `json:"signature"`
}

func (n *Node) SummaryText() string {
	if n.Summary != nil {
		return n.Summary.Text
	}
	return ""
}

func (n *Node) Clone() *Node {
	clone := *n
	if n.Metadata != nil {
		clone.Metadata = make(map[string]any, len(n.Metadata))
		for k, v := range n.Metadata {
			clone.Metadata[k] = v
		}
	}
	if n.Summary != nil {
		clone.Summary = &Summary{
			Text:        n.Summary.Text,
			GeneratedBy: n.Summary.GeneratedBy,
			Model:       n.Summary.Model,
			CreatedAt:   n.Summary.CreatedAt,
			UpdatedAt:   n.Summary.UpdatedAt,
		}
	}
	if n.Methods != nil {
		clone.Methods = make([]Method, len(n.Methods))
		copy(clone.Methods, n.Methods)
	}
	return &clone
}

func (n *Node) GenerateID() string {
	switch n.Type {
	case NodeTypeFunction, NodeTypeMethod:
		return fmt.Sprintf("func_%s_%s_%s:%d", n.Package, n.Name, n.File, n.Line)
	case NodeTypeType, NodeTypeInterface:
		return fmt.Sprintf("type_%s_%s", n.Package, n.Name)
	case NodeTypePackage:
		return fmt.Sprintf("pkg_%s", n.Name)
	case NodeTypeFile:
		return fmt.Sprintf("file_%s", n.Name)
	default:
		return fmt.Sprintf("node_%s_%s", n.Package, n.Name)
	}
}
