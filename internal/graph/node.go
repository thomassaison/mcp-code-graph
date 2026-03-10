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
	Summary   *Summary
	Metadata  map[string]any
}

type Summary struct {
	Text        string `json:"text"`
	GeneratedBy string `json:"generated_by"`
	Model       string `json:"model"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

func (n *Node) SummaryText() string {
	if n.Summary != nil {
		return n.Summary.Text
	}
	return ""
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
