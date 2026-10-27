package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/errcode"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// DepthLimit rejects operations nested deeper than Max. Complexity limits
// alone miss narrow but deep queries (job → stages → photos → job → ...).
// Introspection fields (__schema, __type) are not counted, so tooling works.
type DepthLimit struct{ Max int }

var (
	_ graphql.HandlerExtension        = DepthLimit{}
	_ graphql.OperationContextMutator = DepthLimit{}
)

func (DepthLimit) ExtensionName() string                   { return "DepthLimit" }
func (DepthLimit) Validate(graphql.ExecutableSchema) error { return nil }

func (d DepthLimit) MutateOperationContext(_ context.Context, opCtx *graphql.OperationContext) *gqlerror.Error {
	op := opCtx.Doc.Operations.ForName(opCtx.OperationName)
	if op == nil {
		return nil
	}
	if depth := selectionDepth(op.SelectionSet, opCtx.Doc.Fragments, map[string]bool{}); depth > d.Max {
		err := gqlerror.Errorf("operation has depth %d, which exceeds the limit of %d", depth, d.Max)
		errcode.Set(err, "DEPTH_LIMIT_EXCEEDED")
		return err
	}
	return nil
}

// selectionDepth returns the deepest field nesting in set. visiting guards
// against fragment cycles, which validation rejects but we never trust.
func selectionDepth(set ast.SelectionSet, fragments ast.FragmentDefinitionList, visiting map[string]bool) int {
	deepest := 0
	for _, sel := range set {
		var d int
		switch s := sel.(type) {
		case *ast.Field:
			if len(s.Name) >= 2 && s.Name[:2] == "__" {
				continue
			}
			d = 1 + selectionDepth(s.SelectionSet, fragments, visiting)
		case *ast.InlineFragment:
			d = selectionDepth(s.SelectionSet, fragments, visiting)
		case *ast.FragmentSpread:
			if visiting[s.Name] {
				continue
			}
			frag := fragments.ForName(s.Name)
			if frag == nil {
				continue
			}
			visiting[s.Name] = true
			d = selectionDepth(frag.SelectionSet, fragments, visiting)
			delete(visiting, s.Name)
		}
		deepest = max(deepest, d)
	}
	return deepest
}
