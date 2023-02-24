package internal

import (
	"fmt"
	"github.com/deadsy/sdfx/sdf"
	v2 "github.com/deadsy/sdfx/vec/v2"
	"strings"
	"testing"
)

func preOrderNumSDFs(tree *ReflectTree, pre []int, level int) []int {
	pre = append(pre, len(tree.Children))
	fmt.Printf("%s> %#v\n", strings.Repeat("  ", level), tree.Info.SDF)
	for _, ch := range tree.Children {
		pre = preOrderNumSDFs(ch, pre, level+1)
	}
	return pre
}

func testTree2Common(t *testing.T, d sdf.SDF2, expectedPreOrderNumSDFs []int) {
	tree := NewReflectionSDF(d).GetReflectSDFTree2()
	gotPreOrderNumSDFs := preOrderNumSDFs(tree, []int{}, 0)
	if len(expectedPreOrderNumSDFs) != len(gotPreOrderNumSDFs) {
		t.Fatalf("expected a total of %d nodes in the reflection tree, but got %d",
			len(expectedPreOrderNumSDFs), len(gotPreOrderNumSDFs)) // Count the root node
	}
	for step, expected := range expectedPreOrderNumSDFs {
		got := gotPreOrderNumSDFs[step]
		if expected != got {
			t.Fatalf("expected %d children SDFs at pre-order step %d, but got %d", expected, step, got)
		}
	}
}

func TestReflectTree2Single(t *testing.T) {
	s := sdf.Box2D(v2.Vec{X: 1, Y: 1}, 0.25)
	testTree2Common(t, s, []int{0})
}

func TestReflectTree2Union(t *testing.T) {
	var s sdf.SDF2
	s = sdf.Box2D(v2.Vec{X: 1, Y: 1}, 0.25)
	s2 := sdf.Box2D(v2.Vec{X: 2, Y: 1}, 0.25)
	s = sdf.Union2D(s, s2)
	testTree2Common(t, s, []int{2, 0, 0})
}

func TestReflectTree2MultiLevel(t *testing.T) {
	var s sdf.SDF2
	s = sdf.Box2D(v2.Vec{X: 1, Y: 1}, 0.25)
	s2 := sdf.Box2D(v2.Vec{X: 2, Y: 1}, 0.25)
	s = sdf.Union2D(s, s2)
	s2 = sdf.Box2D(v2.Vec{X: 1, Y: 2}, 0.25)
	s = sdf.Difference2D(s, s2)
	testTree2Common(t, s, []int{2, 2, 0, 0, 0})
}
