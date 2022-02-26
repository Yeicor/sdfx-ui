package internal

import (
	"bytes"
	"encoding/gob"
	"github.com/deadsy/sdfx/sdf"
	"github.com/mitchellh/reflectwalk"
	"reflect"
	"unsafe"
)

// ReflectionSDF provides reflect-based metadata about the SDF hierarchy with the provided root: bounding boxes, etc.
// Remember that reflect is relatively slow and results should be cached.
type ReflectionSDF struct {
	sdf interface{}
}

// NewReflectionSDF is internal: do not use outside this project
func NewReflectionSDF(sdf interface{}) *ReflectionSDF {
	return &ReflectionSDF{sdf: sdf}
}

type ReflectTree struct {
	Info     *SDFNodeMeta
	Children []*ReflectTree
}

func (r *ReflectionSDF) GetReflectTree(targetTypes ...reflect.Type) *ReflectTree {
	return r.getReflectTree(r.sdf, targetTypes...)
}

func (r *ReflectionSDF) getReflectTree(rootSdf interface{}, targetSdfTypes ...reflect.Type) *ReflectTree {
	var res *ReflectTree
	parentToSubRes := map[interface{}]*ReflectTree{}
	uniqueID := 0
	err := reflectwalk.Walk([]interface{}{rootSdf}, /* <-- Wrapper for root to work */
		newTreeSDF2WalkerFunc(func(parents []*SDFNodeMeta, value *SDFNodeMeta) error {
			// Process the full hierarchy
			value.ID = uniqueID
			uniqueID++
			subTree := &ReflectTree{Info: value, Children: []*ReflectTree{}}
			if len(parents) == 0 { // Single root node
				res = subTree
			} else { // Some child
				parentToRegisterTo := parents[len(parents)-1].SDF
				appendTo := parentToSubRes[parentToRegisterTo]
				appendTo.Children = append(appendTo.Children, subTree)
			}
			parentToSubRes[value.SDF] = subTree
			return nil
		}, targetSdfTypes...))
	if err != nil {
		panic(err) // Shouldn't happen
	}
	return res
}

type SDFNodeMeta struct {
	ID    int      // An unique ID for this node (unique for the current tree)
	Level int      // The fake level (it is not consistent across different branches)
	Bb    sdf.Box3 // The cached bounding box (as it can be sent through the network)
	// The following are only available in main renderer mode (can't be sent through the network and needs a code restart to use)
	SDF   interface{}   // The SDF (2D/3D)
	Value reflect.Value // The Value (can be modified!)
}

func init() {
	gob.Register(sdf.Box3{})
}

func (s *SDFNodeMeta) GobEncode() ([]byte, error) {
	buf := &bytes.Buffer{}
	err := gob.NewEncoder(buf).Encode([]interface{}{s.ID, s.Level, s.Bb})
	return buf.Bytes(), err
}

func (s *SDFNodeMeta) GobDecode(bs []byte) error {
	var tmp []interface{}
	err := gob.NewDecoder(bytes.NewReader(bs)).Decode(&tmp)
	if err != nil {
		return nil
	}
	s.ID = tmp[0].(int)
	s.SDF = tmp[1]
	s.Bb = tmp[2].(sdf.Box3)
	return err
}

type treeSDFWalkerFunc struct {
	impl                        func(parents []*SDFNodeMeta, value *SDFNodeMeta) error
	targetSdfTypes              []reflect.Type
	curParents                  []*SDFNodeMeta
	lastFound                   *SDFNodeMeta
	curLevel, minLevelSinceLast int
}

func newTreeSDF2WalkerFunc(impl func(parents []*SDFNodeMeta, value *SDFNodeMeta) error, targetSdfTypes ...reflect.Type) *treeSDFWalkerFunc {
	return &treeSDFWalkerFunc{impl: impl, targetSdfTypes: targetSdfTypes}
}

func (i *treeSDFWalkerFunc) Enter(_ reflectwalk.Location) error {
	i.curLevel++
	return nil
}

func (i *treeSDFWalkerFunc) Exit(_ reflectwalk.Location) error {
	i.curLevel--
	if i.curLevel < i.minLevelSinceLast {
		i.minLevelSinceLast = i.curLevel
	}
	return nil
}

func (i *treeSDFWalkerFunc) Interface(value reflect.Value) error {
	// FIXME: Assumes all SDF2 are always saved as interfaces (but they could be concrete types: pointers, functions, maps, etc.)
	if !value.CanInterface() {
		// HACK: Read-only access to unexported value (Interface() is not allowed due to possible write operations?)
		value = getUnexportedField(value, unsafe.Pointer(value.UnsafeAddr()))
	}
	value = value.Elem() // The internal element of the interface
	for _, tp := range i.targetSdfTypes {
		if value.Type().Implements(tp) {
			return i.handleSDF(value, value.Interface())
		}
	}
	return nil
}

func (i *treeSDFWalkerFunc) handleSDF(value reflect.Value, s interface{}) error {
	// FIXME: Detect and avoid infinite loops (self-references in hierarchy)
	//log.Println("handleSDF:", value.Type(), i.curLevel)

	// Find out which SDF to assign to this SDF2
	if i.lastFound != nil && i.curLevel > i.lastFound.Level && i.minLevelSinceLast > i.lastFound.Level { // Below in hierarchy
		i.curParents = append(i.curParents, i.lastFound)
	} else {
		isAbove := false
		for len(i.curParents) > 0 && i.curLevel <= i.curParents[len(i.curParents)-1].Level { // Above in hierarchy
			isAbove = true
			i.curParents = i.curParents[:len(i.curParents)-1]
		}
		if !isAbove { // At the same Level
			// Nothing to change
		}
	}

	// Extra cached values
	var bb sdf.Box3
	switch tmp := s.(type) {
	case sdf.SDF2:
		bb = sdf.Box3{Min: tmp.BoundingBox().Min.ToV3(-1e-3), Max: tmp.BoundingBox().Max.ToV3(1e-3)}
	case sdf.SDF3:
		bb = tmp.BoundingBox()
	}

	// Record the last found Level and reset minLevelSinceLast
	i.lastFound = &SDFNodeMeta{
		ID:    -1,
		Level: i.curLevel,
		Bb:    bb,
		SDF:   s,
		Value: value,
	}
	i.minLevelSinceLast = i.curLevel + 1 // Will be reset on next iteration to curLevel if going back up the tree

	// Call the implementation
	err := i.impl(i.curParents, i.lastFound)

	return err
}

func getUnexportedField(field reflect.Value, unsafeAddr unsafe.Pointer) reflect.Value {
	return reflect.NewAt(field.Type(), unsafeAddr).Elem()
}
