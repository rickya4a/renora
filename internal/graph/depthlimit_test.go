package graph

import (
	"testing"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func TestSelectionDepth(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  int
	}{
		{"flat", `{ serverInfo }`, 1},
		{"nested", `{ a { b { c } } }`, 3},
		{"widest branch is not deepest", `{ a { b c d } e { f { g { h } } } }`, 4},
		{"fragment spread", `{ a { ...F } } fragment F on T { b { c } }`, 3},
		{"inline fragment adds no level", `{ a { ... on T { b } } }`, 2},
		{"introspection is ignored", `{ __schema { types { fields { type { ofType { name } } } } } serverInfo }`, 1},
		{"fragment cycle terminates", `{ a { ...F } } fragment F on T { b { ...F } }`, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := parser.ParseQuery(&ast.Source{Input: tt.query})
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := selectionDepth(doc.Operations[0].SelectionSet, doc.Fragments, map[string]bool{})
			if got != tt.want {
				t.Errorf("depth = %d, want %d", got, tt.want)
			}
		})
	}
}
