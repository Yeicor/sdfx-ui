package dev

import (
	"github.com/deadsy/sdfx/sdf"
	"image/color"
	"math"
)

//-----------------------------------------------------------------------------
// CONFIGURATION
//-----------------------------------------------------------------------------

// Opt3YUp sets the UP direction to Y+ instead of Z+.
func Opt3YUp(yUp bool) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.s = &invertZ{&swapYZ{r3.s}}
		}
	}
}

// Opt3Cam sets the default transform for the camera (pivot center, angles and distance).
// WARNING: Need to run again the main renderer to apply a change of this option.
func Opt3Cam(camCenter sdf.V3, pitch, yaw, dist float64) Option {
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

// Opt3LightDir sets the light direction for basic lighting simulation (set when Color: true).
// Actually, two lights are simulated (the given one and the opposite one), as part of the surface would be hard to see otherwise
func Opt3LightDir(lightDir sdf.V3) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			r3.lightDir = lightDir.Normalize()
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
	lightDir                                  sdf.V3 // The light's direction for ColorMode: true (simple simulation based on normals)
	// Raycast configuration
	rayScaleAndSigmoid, rayStepScale, rayEpsilon float64
	rayMaxSteps                                  int
}

func newDevRenderer3(s sdf.SDF3) devRendererImpl {
	r := &renderer3{
		s:                  &invertZ{s}, // TODO: fix rendering to use Z+ (instead of Z-) as UP instead of this hack.
		camFOV:             math.Pi / 2, // 90º FOV-X
		surfaceColor:       color.RGBA{R: 255 - 20, G: 255 - 40, B: 255 - 80, A: 255},
		backgroundColor:    color.RGBA{B: 50, A: 255},
		errorColor:         color.RGBA{R: 255, B: 255, A: 255},
		normalEps:          1e-4,
		lightDir:           sdf.V3{X: -1, Y: 1, Z: -1}.Normalize(), // Same as default camera TODO: Follow camera mode?
		rayScaleAndSigmoid: 0,
		rayStepScale:       1,
		rayEpsilon:         0.1,
		rayMaxSteps:        100,
	}
	return r
}

func (r *renderer3) Dimensions() int {
	return 3
}

func (r *renderer3) BoundingBox() sdf.Box3 {
	return r.s.BoundingBox()
}

func (r *renderer3) ColorModes() int {
	// 0: Constant color with basic shading (2 lights and no projected shadows)
	// 1: Normal XYZ as RGB
	return 2
}

func (r *renderer3) Render(args *renderArgs) error {
	// Compute camera matrix and more (once per render)
	args.stateLock.RLock()
	colorModeCopy := args.state.ColorMode
	bounds := args.fullRender.Bounds()
	boundsSize := sdf.V2i{bounds.Size().X, bounds.Size().Y}
	aspectRatio := float64(boundsSize[0]) / float64(boundsSize[1])
	camViewMatrix := args.state.Cam3MatrixNoTranslation()
	camPos := args.state.CamCenter.Add(camViewMatrix.MulPosition(sdf.V3{Y: -args.state.CamDist}))
	camDir := args.state.CamCenter.Sub(camPos).Normalize()
	camFovX := r.camFOV
	camFovY := 2 * math.Atan(math.Tan(camFovX/2)*aspectRatio)
	// Approximate max ray length for the whole camera (it could be improved... or maybe a fixed value is better)
	sBb := r.BoundingBox()
	maxRay := math.Abs(collideRayBb(camPos, camDir, sBb))
	// If we do not hit the box (in a straight line, set a default -- box size, as following condition will be true)
	if !sBb.Contains(camPos) { // If we hit from the outside of the box, add the whole size of the box
		maxRay += sBb.Size().Length()
	}
	maxRay *= 4 // Rays thrown from the camera at different angles may need a little more maxRay
	args.stateLock.RUnlock()

	// Perform the actual render
	return implCommonRender(func(pixel sdf.V2i, pixel01 sdf.V2) interface{} {
		return &pixelRender{
			pixel:         pixel,
			bounds:        boundsSize,
			camPos:        camPos,
			camDir:        camDir,
			camViewMatrix: camViewMatrix,
			camFov:        sdf.V2{X: camFovX, Y: camFovY},
			maxRay:        maxRay,
			color:         colorModeCopy,
			rendered:      color.RGBA{},
		}
	}, func(pixel sdf.V2i, pixel01 sdf.V2, job interface{}) *jobResult {
		return &jobResult{
			pixel: pixel,
			color: r.samplePixel(pixel01, job.(*pixelRender)),
		}
	}, args, &r.pixelsRand)

	// TODO: Draw bounding boxes over the image
}

type pixelRender struct {
	// CAMERA RELATED
	pixel, bounds  sdf.V2i // Pixel and bounds for pixel
	camPos, camDir sdf.V3  // Camera parameters
	camViewMatrix  sdf.M44 // The world to camera matrix
	camFov         sdf.V2  // Camera's field of view
	maxRay         float64 // The maximum distance of a ray (camPos, camDir) before getting out of bounds
	// MISC
	color int
	// OUTPUT
	rendered color.RGBA
}

func (r *renderer3) samplePixel(pixel01 sdf.V2, job *pixelRender) color.RGBA {
	// Generate the ray for this pixel using the given camera parameters
	rayFrom := job.camPos
	// Get pixel inside of ([-1, 1], [-1, 1])
	rayDirXZBase := pixel01.MulScalar(2).SubScalar(1)
	rayDirXZBase.X *= float64(job.bounds[0]) / float64(job.bounds[1]) // Apply aspect ratio (again)
	//rayDirXZBase.Y = -rayDirXZBase.Y
	// Convert to the projection over a displacement of 1
	rayDirXZBase = rayDirXZBase.Mul(sdf.V2{X: math.Tan(job.camFov.DivScalar(2).X), Y: math.Tan(job.camFov.DivScalar(2).Y)})
	rayDir := sdf.V3{X: rayDirXZBase.X, Y: 1, Z: rayDirXZBase.Y} // Z is UP (and this default camera is X-right Y-up)
	// Apply the camera matrix to the default ray
	rayDir = job.camViewMatrix.MulPosition(rayDir) // .Normalize() (done in Raycast already)
	// TODO: Orthogonal camera mode?

	// Query the surface with the given ray
	hit, t, steps := sdf.Raycast3(r.s, rayFrom, rayDir, r.rayScaleAndSigmoid, r.rayStepScale, r.rayEpsilon, job.maxRay, r.rayMaxSteps)
	// Convert the possible hit to a color
	if t >= 0 { // Hit the surface
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
		} else { // Color == abs(normal)
			return color.RGBA{
				R: uint8(math.Abs(normal.X) * 255),
				G: uint8(math.Abs(normal.Y) * 255),
				B: uint8(math.Abs(normal.Z) * 255),
				A: 255,
			}
		}
	} else {
		if steps == r.rayMaxSteps {
			// Reached the maximum amount of steps (should change parameters)
			return r.errorColor
		}
		// The void
		return r.backgroundColor
	}
}

type invertZ struct {
	impl sdf.SDF3
}

func (i *invertZ) Evaluate(p sdf.V3) float64 {
	return i.impl.Evaluate(p.Mul(sdf.V3{X: 1, Y: 1, Z: -1}))
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
func collideRayBb(origin sdf.V3, dir sdf.V3, bb sdf.Box3) float64 {
	dirFrac := sdf.V3{X: 1 / dir.X, Y: 1 / dir.Y, Z: 1 / dir.Z} // Assumes normalized dir
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

func (s *swapYZ) Evaluate(p sdf.V3) float64 {
	return s.impl.Evaluate(sdf.V3{X: p.X, Y: p.Z, Z: p.Y})
}

func (s *swapYZ) BoundingBox() sdf.Box3 {
	box := s.impl.BoundingBox()
	box.Min.Z, box.Min.Y = box.Min.Y, box.Min.Z
	box.Max.Z, box.Max.Y = box.Max.Y, box.Max.Z
	return box
}
