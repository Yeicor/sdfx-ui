package ui

import (
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/sdf"
	v2 "github.com/deadsy/sdfx/vec/v2"
	"github.com/deadsy/sdfx/vec/v2i"
	v3 "github.com/deadsy/sdfx/vec/v3"
	"image"
	"image/color"
	"image/color/palette"
	"math"
)

//-----------------------------------------------------------------------------
// CONFIGURATION
//-----------------------------------------------------------------------------

// Opt2Cam sets the default camera for SDF2 (may grow to follow the aspect ratio of the screen).
// WARNING: Need to run again the main renderer to apply a change of this option.
func Opt2Cam(bb sdf.Box2) Option {
	return func(r *Renderer) {
		r.implState.Bb = bb
	}
}

// Opt2EvalRange skips the initial scan of the SDF2 to find the minimum and maximum value, and can also be used to
// make the surface easier to see by setting them to a value close to 0.
func Opt2EvalRange(min, max float64) Option {
	return func(r *Renderer) {
		if r2, ok := r.impl.(*renderer2); ok {
			r2.evalMin = min
			r2.evalMax = max
		}
	}
}

// Opt2EvalScanCells configures the initial scan of the SDF2 to find minimum and maximum values (defaults to 128x128 cells).
func Opt2EvalScanCells(cells v2i.Vec) Option {
	return func(r *Renderer) {
		if r2, ok := r.impl.(*renderer2); ok {
			r2.evalScanCells = cells
		}
	}
}

// Opt2BBColor sets the bounding box colors for the different objects.
func Opt2BBColor(getColor func(idx int) color.Color) Option {
	return func(r *Renderer) {
		if r2, ok := r.impl.(*renderer2); ok {
			r2.getBBColor = getColor
		}
	}
}

//-----------------------------------------------------------------------------
// RENDERER
//-----------------------------------------------------------------------------

type renderer2 struct {
	s                sdf.SDF2 // The SDF to render
	pixelsRand       []int    // Cached set of pixels in random order to avoid shuffling (reset on recompilation and resolution changes)
	evalMin, evalMax float64  // The pre-computed minimum and maximum of the whole surface (for stable colors and speed)
	evalScanCells    v2i.Vec
	getBBColor       func(idx int) color.Color
}

func newDevRenderer2(s sdf.SDF2) internal.DevRendererImpl {
	r := &renderer2{
		s:             s,
		evalScanCells: v2i.Vec{128, 128},
		getBBColor: func(idx int) color.Color {
			return palette.WebSafe[((idx + 1) % len(palette.WebSafe))]
		},
	}
	return r
}

func (r *renderer2) Dimensions() int {
	return 2
}

func (r *renderer2) BoundingBox() sdf.Box3 {
	bb := r.s.BoundingBox()
	return sdf.Box3{Min: v3.Vec{X: bb.Min.X, Y: bb.Max.Y, Z: 0.}, Max: v3.Vec{X: bb.Max.X, Y: bb.Max.Y, Z: 0.}}
}

func (r *renderer2) ReflectTree() *internal.ReflectTree {
	return internal.NewReflectionSDF(r.s).GetReflectSDFTree2()
}

func (r *renderer2) ColorModes() int {
	// 0: Gradient (useful for debugging sides)
	// 1: Black/white (clearer surface boundary)
	return 2
}

func (r *renderer2) Render(args *internal.RenderArgs) error {
	if r.evalMin == 0 && r.evalMax == 0 { // First render (ignoring external cache)
		// Compute minimum and maximum evaluate values for a shared color scale for all blocks
		r.evalMin, r.evalMax = utilSdf2MinMax(r.s, r.s.BoundingBox(), r.evalScanCells)
		//log.Println("MIN:", r.evalMin, "MAX:", r.evalMax)
	}

	// Maintain Bb aspect ratio on ResInv change, increasing the sizeCorner as needed
	args.StateLock.Lock()
	fullRenderSize := args.FullRender.Bounds().Size()
	bbAspectRatio := args.State.Bb.Size().X / args.State.Bb.Size().Y
	screenAspectRatio := float64(fullRenderSize.X) / float64(fullRenderSize.Y)
	if math.Abs(bbAspectRatio-screenAspectRatio) > 1e-12 {
		if bbAspectRatio > screenAspectRatio {
			scaleYBy := bbAspectRatio / screenAspectRatio
			args.State.Bb = sdf.NewBox2(args.State.Bb.Center(), args.State.Bb.Size().Mul(v2.Vec{X: 1, Y: scaleYBy}))
		} else {
			scaleXBy := screenAspectRatio / bbAspectRatio
			args.State.Bb = sdf.NewBox2(args.State.Bb.Center(), args.State.Bb.Size().Mul(v2.Vec{X: scaleXBy, Y: 1}))
		}
	}
	args.StateLock.Unlock()

	// Apply color mode
	evalMin, evalMax := r.evalMin, r.evalMax
	if args.State.ColorMode == 1 { // Force black and white to see the surface better
		evalMin, evalMax = -1e-12, 1e-12
	}

	// Perform the actual render
	err := implCommonRender(func(pixel v2i.Vec, pixel01 v2.Vec) interface{} { return nil },
		func(pixel v2i.Vec, pixel01 v2.Vec, job interface{}) *jobResult {
			pixel01.Y = 1 - pixel01.Y // Inverted Y
			args.StateLock.RLock()
			pos := args.State.Bb.Min.Add(pixel01.Mul(args.State.Bb.Size()))
			args.StateLock.RUnlock()
			grayVal := imageColor2(r.s.Evaluate(pos), evalMin, evalMax)
			return &jobResult{
				pixel: pixel,
				color: color.RGBA{R: uint8(grayVal * 255), G: uint8(grayVal * 255), B: uint8(grayVal * 255), A: 255},
			}
		}, args, &r.pixelsRand)

	if err == nil && args.State.DrawBbs {
		// Draw bounding boxes over the image
		boxes2 := args.State.ReflectTree.GetBoundingBoxes2()
		for i, bb := range boxes2 {
			//log.Println("Draw", bb)
			pixel01Min := bb.Min.Sub(args.State.Bb.Min).Div(args.State.Bb.Size())
			pixel01Max := bb.Max.Sub(args.State.Bb.Min).Div(args.State.Bb.Size())
			fullRenderSizeV2 := v2.Vec{X: float64(fullRenderSize.X), Y: float64(fullRenderSize.Y)}
			posMin := pixel01Min.Mul(fullRenderSizeV2)
			posMax := pixel01Max.Mul(fullRenderSizeV2)
			drawRect(args.FullRender, int(posMin.X), fullRenderSize.Y-int(posMax.Y),
				int(posMax.X), fullRenderSize.Y-int(posMin.Y), r.getBBColor(i))
		}
	}

	return err
}

// imageColor2 returns the grayscale color for the returned SDF2.Evaluate value, given the reference minimum and maximum
// SDF2.Evaluate values. The returned value is in the range [0, 1].
func imageColor2(dist, dmin, dmax float64) float64 {
	// Clamp due to possibly forced min and max
	var val float64
	// NOTE: This condition forces the surface to be close to 255/2 gray value, otherwise dmax >>> dmin or viceversa
	// could cause the surface to be visually displaced
	if dist >= 0 {
		val = math.Max(0.5, math.Min(1, 0.5+0.5*((dist)/(dmax))))
	} else { // Force lower scale for inside surface
		val = math.Max(0, math.Min(0.5, 0.5*((dist-dmin)/(-dmin))))
	}
	return val
}

// drawHLine draws a horizontal line
func drawHLine(img *image.RGBA, x1, y, x2 int, col color.Color) {
	for ; x1 <= x2; x1++ {
		if x1 >= 0 && x1 < img.Bounds().Dx() && y >= 0 && y < img.Bounds().Dy() {
			img.Set(x1, y, col)
		}
	}
}

// drawVLine draws a veritcal line
func drawVLine(img *image.RGBA, x, y1, y2 int, col color.Color) {
	for ; y1 <= y2; y1++ {
		if x >= 0 && x < img.Bounds().Dx() && y1 >= 0 && y1 < img.Bounds().Dy() {
			img.Set(x, y1, col)
		}
	}
}

// drawRect draws a rectangle utilizing drawHLine() and drawVLine()
func drawRect(img *image.RGBA, x1, y1, x2, y2 int, col color.Color) {
	drawHLine(img, x1, y1, x2, col)
	drawHLine(img, x1, y2, x2, col)
	drawVLine(img, x1, y1, y2, col)
	drawVLine(img, x2, y1, y2, col)
}
