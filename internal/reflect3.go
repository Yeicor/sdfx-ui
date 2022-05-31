package internal

import (
	"github.com/deadsy/sdfx/sdf"
	"reflect"
)

var sdf3Type = reflect.TypeOf((*sdf.SDF3)(nil)).Elem()

// GetReflectSDFTree3 is internal: do not use outside this project
func (r *ReflectionSDF) GetReflectSDFTree3() *ReflectTree {
	// NOTE: The SDF3 hierarchy may also contain SDF2 (most likely for initial 2D design that is later extruded)
	// TODO: Include SDF2 boxes? Works, but results in ugly renderings
	return r.GetReflectTree( /*sdf2Type, */ sdf3Type)
}

// GetBoundingBoxes3 is internal: do not use outside this project
func (r *ReflectTree) GetBoundingBoxes3() []sdf.Box3 {
	return r.getBoundingBoxes3(r)
}

// getBoundingBoxes3 flattens the tree if only bounds are wanted
func (r *ReflectTree) getBoundingBoxes3(tree *ReflectTree) []sdf.Box3 {
	var res []sdf.Box3
	// HACK: Skip condition: to make results cleaner
	skipParent := false
	// HACK: Stop condition (apart from finishing the tree): to make results cleaner
	skipChildren := false
	if tree.Info.Value.Kind() != reflect.Invalid && !tree.Info.Value.IsNil() {
		tpName := tree.Info.Value.Type().String()
		switch tpName {
		case "*ui.swapYZ":
			fallthrough
		case "*ui.invertZ":
			skipParent = true
		case "*sdf.TransformSDF3":
			fallthrough
		case "*sdf.ScaleUniformSDF3":
			skipChildren = true
		default:
		}
	}
	if !skipParent {
		res = append(res, tree.Info.Bb)
	}
	if !skipChildren {
		for _, subTree := range tree.Children {
			res = append(res, r.getBoundingBoxes3(subTree)...)
		}
	}
	return res
}
