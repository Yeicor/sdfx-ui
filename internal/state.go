package internal

import (
	"context"
	"github.com/deadsy/sdfx/sdf"
	"image"
	"sync"
)

// DevRendererImpl is the interface implemented by the SDF2 and SDF3 renderers.
// Note that the implementation is independent of the graphics backend used and renders CPU images.
type DevRendererImpl interface {
	// Dimensions are 2 for SDF2 and 3 for SDF3
	Dimensions() int
	// BoundingBox returns the full bounding box of the surface (Z is ignored for SDF2)
	BoundingBox() sdf.Box3
	// ReflectTree returns the main reflection-based metadata structure for handling the SDF hierarchy
	ReflectTree() *ReflectTree
	// ColorModes returns the number of color modes supported
	ColorModes() int
	// Render performs a full render, given the screen size (it may be cancelled using the given context).
	// Returns partially rendered images as progress is made through PartialRenders (if non-nil, channel closed).
	Render(args *RenderArgs) error
	// TODO: Map clicks to source code? (using reflection on the SDF and profiling/code generation?)
}

// RendererState is an internal struct that has to be exported for RPC.
type RendererState struct {
	// SHARED
	ResInv      int          // How detailed is the image: number screen pixels for each pixel rendered (SDF2: use a power of two)
	DrawBbs     bool         // Whether to show all bounding boxes (useful for debugging subtraction/intersection of SDFs)
	ColorMode   int          // The color mode (each render may support multiple modes)
	ReflectTree *ReflectTree // Cached read-only reflection metadata to have some insight into the SDF hierarchy
	// SDF2
	Bb sdf.Box2 // Controls the scale and displacement
	// SDF3
	CamCenter                 sdf.V3  // Arc-Ball camera center (the point we are looking at)
	CamYaw, CamPitch, CamDist float64 // Arc-Ball rotation angles (around CamCenter) and distance from CamCenter
}

type RenderArgs struct {
	Ctx                         context.Context
	State                       *RendererState
	StateLock, CachedRenderLock *sync.RWMutex
	PartialRenders              chan<- *image.RGBA
	FullRender                  *image.RGBA
}
