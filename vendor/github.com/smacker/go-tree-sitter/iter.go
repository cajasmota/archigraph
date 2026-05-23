//go:build cgo

// PATCHED FOR ARCHIGRAPH #1796 (2026-05-23):
// Added "//go:build cgo" because iter.go references *Node which is only
// defined in bindings.go (CGO-tagged). Without this tag, Windows go vet
// without MinGW fails with `undefined: Node`. Upstream smacker/go-tree-sitter
// has not fixed this in any cached version. When upstream releases a fix,
// remove this patch and re-vendor.

package sitter

import "io"

type IterMode int

const (
	DFSMode IterMode = iota
	BFSMode
)

// Iterator for a tree of nodes
type Iterator struct {
	named bool
	mode  IterMode

	nodesToVisit []*Node
}

// NewIterator takes a node and mode (DFS/BFS) and returns iterator over children of the node
func NewIterator(n *Node, mode IterMode) *Iterator {
	return &Iterator{
		named:        false,
		mode:         mode,
		nodesToVisit: []*Node{n},
	}
}

// NewNamedIterator takes a node and mode (DFS/BFS) and returns iterator over named children of the node
func NewNamedIterator(n *Node, mode IterMode) *Iterator {
	return &Iterator{
		named:        true,
		mode:         mode,
		nodesToVisit: []*Node{n},
	}
}

func (iter *Iterator) Next() (*Node, error) {
	if len(iter.nodesToVisit) == 0 {
		return nil, io.EOF
	}

	var n *Node
	n, iter.nodesToVisit = iter.nodesToVisit[0], iter.nodesToVisit[1:]

	var children []*Node
	if iter.named {
		for i := 0; i < int(n.NamedChildCount()); i++ {
			children = append(children, n.NamedChild(i))
		}
	} else {
		for i := 0; i < int(n.ChildCount()); i++ {
			children = append(children, n.Child(i))
		}
	}

	switch iter.mode {
	case DFSMode:
		iter.nodesToVisit = append(children, iter.nodesToVisit...)
	case BFSMode:
		iter.nodesToVisit = append(iter.nodesToVisit, children...)
	default:
		panic("not implemented")
	}
	return n, nil
}

func (iter *Iterator) ForEach(fn func(*Node) error) error {
	for {
		n, err := iter.Next()
		if err != nil {
			return err
		}
		err = fn(n)
		if err != nil {
			return err
		}
	}
}
