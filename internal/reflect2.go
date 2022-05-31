package internal

import (
	"github.com/deadsy/sdfx/sdf"
	"reflect"
)

var sdf2Type = reflect.TypeOf((*sdf.SDF2)(nil)).Elem()

// GetReflectSDFTree2 is internal: do not use outside this project
func (r *ReflectionSDF) GetReflectSDFTree2() *ReflectTree {
	return r.GetReflectTree(sdf2Type)
}

// GetBoundingBoxes2 is internal: do not use outside this project
func (r *ReflectTree) GetBoundingBoxes2() []sdf.Box2 {
	return r.getBoundingBoxes2(r)
}

// getBoundingBoxes2 flattens the tree if only bounds are wanted
func (r *ReflectTree) getBoundingBoxes2(tree *ReflectTree) []sdf.Box2 {
	var res []sdf.Box2
	// HACK: Skip condition: to make results cleaner
	skipParent := false
	// HACK: Stop condition (apart from finishing the tree): to make results cleaner
	skipChildren := false
	if !tree.Info.Value.IsNil() {
		tpName := tree.Info.Value.Type().String()
		switch tpName {
		//case "*ui.swapYZ":
		//	fallthrough
		//case "*ui.invertZ":
		//	skipParent = true
		case "*sdf.TransformSDF2":
			fallthrough
		case "*sdf.ScaleUniformSDF2":
			skipChildren = true
		default:
		}
	}
	if //goland:noinspection GoBoolExpressions
	!skipParent {
		res = append(res, sdf.Box2{
			Min: sdf.V2{X: tree.Info.Bb.Min.X, Y: tree.Info.Bb.Min.Y},
			Max: sdf.V2{X: tree.Info.Bb.Max.X, Y: tree.Info.Bb.Max.Y},
		})
	}
	if !skipChildren {
		for _, subTree := range tree.Children {
			res = append(res, r.getBoundingBoxes2(subTree)...)
		}
	}
	return res
}
