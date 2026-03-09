package graph

type EdgeType string

const (
	EdgeTypeCalls      EdgeType = "calls"
	EdgeTypeCalledBy   EdgeType = "called_by"
	EdgeTypeImports    EdgeType = "imports"
	EdgeTypeUses       EdgeType = "uses"
	EdgeTypeDefines    EdgeType = "defines"
	EdgeTypeImplements EdgeType = "implements"
	EdgeTypeEmbeds     EdgeType = "embeds"
	EdgeTypeReturns    EdgeType = "returns"
	EdgeTypeAccepts    EdgeType = "accepts"
)

type Edge struct {
	From     string
	To       string
	Type     EdgeType
	Metadata map[string]any
}
