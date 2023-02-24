package ui

import (
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/sdf"
	"github.com/deadsy/sdfx/vec/v2"
	"github.com/deadsy/sdfx/vec/v2i"
	"github.com/deadsy/sdfx/vec/v3"
	"image"
	"image/color"
	"image/color/palette"
	"math"
)

//-----------------------------------------------------------------------------
// CONFIGURATION
//-----------------------------------------------------------------------------

// Opt3SwapYAndZ sets the UP direction to Y+ instead of Z+ (or swaps it back).
func Opt3SwapYAndZ() Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.s = &swapYZ{r3.s}
		}
	}
}

// Opt3Cam sets the default transform for the camera (pivot center, angles and distance).
// WARNING: Need to run again the main renderer to apply a change of this option.
func Opt3Cam(camCenter v3.Vec, pitch, yaw, dist float64) Option {
	return func(r *Renderer) {
		r.implState.CamCenter = camCenter
		r.implState.CamPitch = pitch
		r.implState.CamYaw = yaw
		r.implState.CamDist = dist
	}
}

// Opt3CamFov sets the default Field Of View for the camera (default 90º, in radians).
func Opt3CamFov(fov float64) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.camFOV = fov
		}
	}
}

// Opt3RayConfig sets the configuration for the raycast (balancing performance and quality).
// Rendering a pink pixel means that the ray reached maxSteps without hitting the surface or reaching the limit
// (consider increasing maxSteps (reduce performance), increasing epsilon or increasing stepScale (both reduce quality)).
func Opt3RayConfig(scaleAndSigmoid, stepScale, epsilon float64, maxSteps int) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.rayScaleAndSigmoid = scaleAndSigmoid
			r3.rayStepScale = stepScale
			r3.rayEpsilon = epsilon
			r3.rayMaxSteps = maxSteps
		}
	}
}

// Opt3Colors changes rendering colors.
func Opt3Colors(surface, background, error color.RGBA) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.surfaceColor = surface
			r3.backgroundColor = background
			r3.errorColor = error
		}
	}
}

// Opt3NormalEps sets the distance between samples used to compute the normals.
func Opt3NormalEps(normalEps float64) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.normalEps = normalEps / 2
		}
	}
}

// Opt3LightDir sets the light direction for basic lighting simulation.
// Actually, two lights are simulated (the given one and the opposite one), as part of the surface would be hard to see otherwise
func Opt3LightDir(lightDir v3.Vec) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.lightDir = lightDir.Normalize()
		}
	}
}

// Opt3BBColor sets the bounding box colors for the different objects.
func Opt3BBColor(getColor func(idx int) color.Color) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.getBBColor = getColor
		}
	}
}

//-----------------------------------------------------------------------------
// RENDERER
//-----------------------------------------------------------------------------

type renderer3 struct {
	s                                         sdf.SDF3 // The SDF to render
	pixelsRand                                []int    // Cached set of pixels in random order to avoid shuffling (reset on recompilation and resolution changes)
	camFOV                                    float64  // The Field Of View (X axis) for the camera
	surfaceColor, backgroundColor, errorColor color.RGBA
	normalEps                                 float64
	lightDir                                  v3.Vec // The light's direction for ColorMode: true (simple simulation based on normals)
	depthBuffer                               []float64
	getBBColor                                func(idx int) color.Color

	// Raycast configuration
	rayScaleAndSigmoid, rayStepScale, rayEpsilon float64
	rayMaxSteps                                  int

	meshRenderer *renderer3mesh // Alternative renderer
}

func newDevRenderer3(s sdf.SDF3) internal.DevRendererImpl {
	r := &renderer3{
		s:                  &invertZ{s}, // TODO: fix rendering to use Z+ (instead of Z-) as UP instead of this hack.
		camFOV:             math.Pi / 2, // 90º FOV-X
		surfaceColor:       color.RGBA{R: 255 - 20, G: 255 - 40, B: 255 - 80, A: 255},
		backgroundColor:    color.RGBA{R: 50, G: 100, B: 150, A: 255},
		errorColor:         color.RGBA{R: 255, B: 255, A: 255},
		normalEps:          1e-6,
		lightDir:           v3.Vec{X: -1, Y: 1, Z: 1}.Normalize(), // Same as default camera TODO: Follow camera mode?
		rayScaleAndSigmoid: 0,
		rayStepScale:       1,
		rayEpsilon:         1e-2,
		rayMaxSteps:        100,
		meshRenderer:       &renderer3mesh{},
		getBBColor: func(idx int) color.Color {
			return palette.WebSafe[((idx + 1) % len(palette.WebSafe))]
		},
	}
	return r
}

func (r *renderer3) Dimensions() int {
	return 3
}

func (r *renderer3) BoundingBox() sdf.Box3 {
	return r.s.BoundingBox()
}

func (r *renderer3) ReflectTree() *internal.ReflectTree {
	return internal.NewReflectionSDF(r.s).GetReflectSDFTree3()
}

func (r *renderer3) ColorModes() int {
	// Use alternative renderer instead if configured to do so
	if r.meshRenderer != nil {
		return r.meshRenderer.ColorModes()
	}
	// 0: Constant color with basic shading (2 lights and no projected shadows)
	// 1: Normal XYZ as RGB
	return 2
}

func (r *renderer3) Render(args *internal.RenderArgs) error {
	// Use alternative renderer instead if configured to do so
	if r.meshRenderer != nil && r.meshRenderer.mesh != nil {
		err := r.meshRenderer.Render(r, args)
		return err
	}

	// Compute camera matrix and more (once per render)
	args.StateLock.RLock()
	colorModeCopy := args.State.ColorMode
	bounds := args.FullRender.Bounds()
	boundsSize := v2i.Vec{bounds.Size().X, bounds.Size().Y}
	//aspectRatio := float64(boundsSize[0]) / float64(boundsSize.Y)
	camViewMatrix := cam3MatrixNoTranslation(args.State)
	camPos := args.State.CamCenter.Add(camViewMatrix.MulPosition(v3.Vec{Y: -args.State.CamDist}))
	camDir := args.State.CamCenter.Sub(camPos).Normalize()
	camFovX := r.camFOV
	camFovY := 2 * math.Atan(math.Tan(camFovX/2) /**aspectRatio*/)
	// Approximate max ray length for the whole camera (it could be improved... or maybe a fixed value is better)
	sBb := r.BoundingBox()
	maxRay := math.Abs(collideRayBb(camPos, camDir, sBb))
	// If we do not hit the box (in a straight line, set a default -- box size, as following condition will be true)
	if !sBb.Contains(camPos) { // If we hit from the outside of the box, add the whole size of the box
		maxRay += sBb.Size().Length()
	}
	maxRay *= 4 // Rays thrown from the camera at different angles may need a little more maxRay

	if args.State.DrawBbs {
		// Reset internal depth buffer
		expectedLen := boundsSize.X * boundsSize.Y
		if len(r.depthBuffer) != expectedLen {
			r.depthBuffer = make([]float64, expectedLen)
		}
		for i := 0; i < len(r.depthBuffer); i++ {
			r.depthBuffer[i] = math.MaxFloat64
		}
	} else {
		r.depthBuffer = nil
	}
	args.StateLock.RUnlock()

	// Perform the actual render
	camHalfFov := v2.Vec{X: camFovX, Y: camFovY}.DivScalar(2)
	err := implCommonRender(func(pixel v2i.Vec, pixel01 v2.Vec) interface{} {
		return &pixelRender{
			pixel:         pixel,
			bounds:        boundsSize,
			camPos:        camPos,
			camDir:        camDir,
			camViewMatrix: camViewMatrix,
			camHalfFov:    camHalfFov,
			maxRay:        maxRay,
			color:         colorModeCopy,
			rendered:      color.RGBA{},
		}
	}, func(pixel v2i.Vec, pixel01 v2.Vec, job interface{}) *jobResult {
		return &jobResult{
			pixel: pixel,
			color: r.samplePixel(pixel01, job.(*pixelRender)),
		}
	}, args, &r.pixelsRand)

	if err == nil && args.State.DrawBbs {
		// FIXME: Assumes perfectly matching cameras (between both 3D renderers),
		//  but they differ (in aspect ratio <--> FoV, matching on square windows)
		r.renderBbs(args, r.depthBuffer)
	}

	return err
}

func (r *renderer3) renderBbs(args *internal.RenderArgs, depthBuffer []float64) {
	// Needed to render boxes
	backgroundColorOld := r.backgroundColor
	r.backgroundColor = color.RGBA{A: 0}
	camMatrix, _ := r.meshRenderer.reset(r, args)
	r.backgroundColor = backgroundColorOld
	// Draw bounding boxes over the image
	var boxesRender *image.NRGBA
	for i, bb := range args.State.ReflectTree.GetBoundingBoxes3() {
		boxesRender = r.meshRenderer.renderBoundingBox(bb, camMatrix, r.getBBColor(i))
	}
	if boxesRender != nil && len(depthBuffer) > 0 {
		// Now merge both renders by depth!
		size := args.FullRender.Bounds().Size()
		boxesDepth := r.meshRenderer.depthBuffer()
		i := 0
		for y := 0; y < size.Y; y++ {
			for x := 0; x < size.X; x++ {
				overlay := boxesRender.NRGBAAt(x, y)
				if overlay.A > 0 {
					boxesRenderDepth := boxesDepth[i]
					renderDepth := depthBuffer[i]
					//if renderDepth < math.MaxFloat64/2 {
					//	fmt.Println(boxesRenderDepth, renderDepth)
					//}
					if boxesRenderDepth < renderDepth {
						//prevColor := args.FullRender.RGBAAt(x, y)
						args.FullRender.Set(x, y, overlay)
					}
				}
				i++
			}
		}
	}
}

type pixelRender struct {
	// CAMERA RELATED
	pixel, bounds  v2i.Vec // Pixel and bounds for pixel
	camPos, camDir v3.Vec  // Camera parameters
	camViewMatrix  sdf.M44 // The world to camera matrix
	camHalfFov     v2.Vec  // Camera's field of view
	maxRay         float64 // The maximum distance of a ray (camPos, camDir) before getting out of bounds
	// MISC
	color int
	// OUTPUT
	rendered color.RGBA
}

func (r *renderer3) samplePixel(pixel01 v2.Vec, job *pixelRender) color.RGBA {
	depthBufferIndex := -1
	if len(r.depthBuffer) > 0 {
		depthBufferIndex = job.pixel.Y*job.bounds.X + job.pixel.X
	}
	// Generate the ray for this pixel using the given camera parameters
	rayFrom := job.camPos
	// Get pixel inside of ([-1, 1], [-1, 1])
	rayDirXZBase := pixel01.MulScalar(2).SubScalar(1)
	rayDirXZBase.Y = -rayDirXZBase.Y
	rayDirXZBase.X *= float64(job.bounds.X) / float64(job.bounds.Y) // Apply aspect ratio (again)
	// Convert to the projection over a displacement of 1
	rayDirXZBase = rayDirXZBase.Mul(v2.Vec{X: math.Tan(job.camHalfFov.X), Y: math.Tan(job.camHalfFov.Y)})
	rayDir := v3.Vec{X: rayDirXZBase.X, Y: 1, Z: rayDirXZBase.Y} // Z is UP (and this default camera is X-right Y-up)
	// Apply the camera matrix to the default ray
	rayDir = job.camViewMatrix.MulPosition(rayDir) // .Normalize() (done in Raycast already)
	// TODO: Orthogonal camera mode?

	// Query the surface with the given ray
	hit, t, steps := sdf.Raycast3(r.s, rayFrom, rayDir, r.rayScaleAndSigmoid, r.rayStepScale, r.rayEpsilon, job.maxRay, r.rayMaxSteps)
	// Convert the possible hit to a color
	if t >= 0 { // Hit the surface
		if len(r.depthBuffer) > 0 { // HACK: Depth function similar to fauxgl (but not the same)
			r.depthBuffer[depthBufferIndex] = 1 / (1 + math.Exp(-t/10))
		}
		normal := sdf.Normal3(r.s, hit, r.normalEps)
		if job.color == 0 { // Basic lighting + constant color
			lightIntensity := math.Abs(normal.Dot(r.lightDir)) // Actually also simulating the opposite light
			// If this was a performant ray-tracer, we could bounce the light
			return color.RGBA{
				R: uint8(float64(r.surfaceColor.R) * lightIntensity),
				G: uint8(float64(r.surfaceColor.G) * lightIntensity),
				B: uint8(float64(r.surfaceColor.B) * lightIntensity),
				A: r.surfaceColor.A,
			}
		} // Otherwise, Color == abs(normal)
		return color.RGBA{
			R: uint8(math.Abs(normal.X) * 255),
			G: uint8(math.Abs(normal.Y) * 255),
			B: uint8(math.Abs(normal.Z) * 255),
			A: 255,
		}
	} else // Otherwise, missed the surface (or run out of steps)
	if len(r.depthBuffer) > 0 {
		r.depthBuffer[depthBufferIndex] = math.MaxFloat64
	}
	if steps == r.rayMaxSteps {
		// Reached the maximum amount of steps (should change parameters)
		return r.errorColor
	}
	// The void
	return r.backgroundColor
}

type invertZ struct {
	impl sdf.SDF3
}

func (i *invertZ) Evaluate(p v3.Vec) float64 {
	return i.impl.Evaluate(p.Mul(v3.Vec{X: 1, Y: 1, Z: -1}))
}

func (i *invertZ) BoundingBox() sdf.Box3 {
	box := i.impl.BoundingBox()
	box.Min.Z = -box.Min.Z
	box.Max.Z = -box.Max.Z
	if box.Max.Z < box.Min.Z {
		box.Max.Z, box.Min.Z = box.Min.Z, box.Max.Z
	}
	return box
}

// collideRayBb https://gamedev.stackexchange.com/a/18459.
// Returns the length traversed through the array to reach the box, which may be negative (hit backwards).
// In case of no hit it returns a guess of where it would hit
func collideRayBb(origin v3.Vec, dir v3.Vec, bb sdf.Box3) float64 {
	dirFrac := v3.Vec{X: 1 / dir.X, Y: 1 / dir.Y, Z: 1 / dir.Z} // Assumes normalized dir
	t135 := bb.Min.Sub(origin).Mul(dirFrac)
	t246 := bb.Max.Sub(origin).Mul(dirFrac)
	tmin := math.Max(math.Max(math.Min(t135.X, t246.X), math.Min(t135.Y, t246.Y)), math.Min(t135.Z, t246.Z))
	tmax := math.Min(math.Min(math.Max(t135.X, t246.X), math.Max(t135.Y, t246.Y)), math.Max(t135.Z, t246.Z))
	//if tmin > tmax { // if tmin > tmax, ray doesn't intersect AABB
	//	return inf
	//}
	if tmax < 0 { // if tmax < 0, ray (line) is intersecting AABB, but the whole AABB is behind us
		return tmax
	}
	if bb.Contains(origin) { // This is triggered if inside
		return tmax
	}
	return tmin
}

type swapYZ struct {
	impl sdf.SDF3
}

func (s *swapYZ) Evaluate(p v3.Vec) float64 {
	return s.impl.Evaluate(v3.Vec{X: p.X, Y: p.Z, Z: p.Y})
}

func (s *swapYZ) BoundingBox() sdf.Box3 {
	box := s.impl.BoundingBox()
	box.Min.Z, box.Min.Y = box.Min.Y, box.Min.Z
	box.Max.Z, box.Max.Y = box.Max.Y, box.Max.Z
	return box
}
