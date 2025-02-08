package ui

import (
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/render"
	"github.com/deadsy/sdfx/sdf"
	"github.com/deadsy/sdfx/vec/v2i"
	v3 "github.com/deadsy/sdfx/vec/v3"
	"github.com/fogleman/fauxgl"
	"image"
	"image/color"
	"log"
	"math"
)

//-----------------------------------------------------------------------------
// CONFIGURATION
//-----------------------------------------------------------------------------

// Opt3Mesh enables and configures the 3D mesh renderer instead of the default raycast based renderer
// WARNING: Should be the last option applied (as some other options might modify the SDF3).
func Opt3Mesh(meshGenerator render.Render3, smoothNormalsRadians float64) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			log.Println("[DevRenderer] Rendering 3D mesh...") // only performed once per compilation
			var triangles []*fauxgl.Triangle
			triChan := make(chan []*sdf.Triangle3)
			go func() {
				meshGenerator.Render(r3.s, triChan)
				close(triChan)
			}()
			for tris := range triChan {
				for _, tri := range tris {
					triangles = append(triangles, r3mConvertTriangle(tri))
				}
			}
			mesh := fauxgl.NewTriangleMesh(triangles)
			// smooth the normals
			mesh.SmoothNormalsThreshold(smoothNormalsRadians)
			r3.meshRenderer = &renderer3mesh{mesh: mesh, lastContext: nil}
			log.Println("[DevRenderer] Mesh is ready")
		}
	}
}

//-----------------------------------------------------------------------------
// RENDERER
//-----------------------------------------------------------------------------

// renderer3mesh is an extension to renderer3 that is set when the trimesh renderer is enabled
type renderer3mesh struct {
	mesh        *fauxgl.Mesh // the pre-compiled mesh to render
	lastContext *fauxgl.Context
}

func (rm *renderer3mesh) ColorModes() int {
	// 0: Constant color with basic shading (1 light and no projected shadows)
	// 1: Normal XYZ as RGB
	// 2: 1 but in wireframe mode
	return 3
}

func (rm *renderer3mesh) Render(r *renderer3, args *internal.RenderArgs) error {
	camFauxglMatrix, camPos := rm.reset(r, args)

	// Configure the shader (based on ColorMode)
	if args.State.ColorMode == 0 {
		// use builtin phong shader
		shader := fauxgl.NewPhongShader(camFauxglMatrix, r3mToFauxglVector(r.lightDir), r3mToFauxglVector(camPos))
		shader.ObjectColor = fauxgl.MakeColor(r.surfaceColor)
		rm.lastContext.Shader = shader
		rm.lastContext.Wireframe = false
	} else {
		// use normal based shader
		rm.lastContext.Shader = &r3mNormalShader{camFauxglMatrix}
		rm.lastContext.Wireframe = args.State.ColorMode == 2 // set to wireframe mode
	}
	// Perform the actual render
	rm.lastContext.DrawMesh(rm.mesh) // This is already multithread, no need to parallelize anymore
	img := rm.lastContext.Image()

	// Copy output full render (no partial renders supported)
	args.CachedRenderLock.Lock()
	copy(args.FullRender.Pix[args.FullRender.PixOffset(0, 0):], img.(*image.NRGBA).Pix[img.(*image.NRGBA).PixOffset(0, 0):])
	args.CachedRenderLock.Unlock()

	if args.State.DrawBbs {
		// Draw bounding boxes over the image
		depthBufferClone := make([]float64, len(rm.lastContext.DepthBuffer))
		copy(depthBufferClone, rm.lastContext.DepthBuffer)
		r.renderBbs(args, depthBufferClone)
	}

	if args.PartialRenders != nil {
		close(args.PartialRenders)
	}

	return nil
}

func (rm *renderer3mesh) reset(r *renderer3, args *internal.RenderArgs) (fauxgl.Matrix, v3.Vec) {
	args.StateLock.Lock()
	bounds := args.FullRender.Bounds()
	boundsSize := v2i.Vec{bounds.Size().X, bounds.Size().Y}
	if rm.lastContext == nil || rm.lastContext.Width != boundsSize.X || rm.lastContext.Height != boundsSize.Y {
		// Rebuild rendering context only when needed
		rm.lastContext = fauxgl.NewContext(boundsSize.X, boundsSize.Y)
	} else {
		rm.lastContext.ClearDepthBuffer()
	}
	rm.lastContext.ClearColorBufferWith(fauxgl.MakeColor(r.backgroundColor))

	// Compute camera matrix and more (once per render)
	//args.state.CamYaw += math.Pi // HACK
	//args.state.CamCenter.X = -args.state.CamCenter.X
	//args.state.CamCenter.Y = -args.state.CamCenter.Y
	aspectRatio := float64(boundsSize.X) / float64(boundsSize.Y)
	camViewMatrix := cam3MatrixNoTranslation(args.State)
	camPos := args.State.CamCenter.Add(camViewMatrix.MulPosition(v3.Vec{Y: -args.State.CamDist / 1.12 /* Adjust to other implementation*/}))
	camDir := args.State.CamCenter.Sub(camPos).Normalize()
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
	camFauxglMatrix := fauxgl.LookAt(r3mToFauxglVector(camPos), r3mToFauxglVector(args.State.CamCenter), fauxgl.Vector{Z: 1}).
		Perspective(camFovY*180/math.Pi, aspectRatio, 1e-6, maxRay)
	//args.state.CamYaw -= math.Pi // HACK (restore)
	//args.state.CamCenter.X = -args.state.CamCenter.X
	//args.state.CamCenter.Y = -args.state.CamCenter.Y
	args.StateLock.Unlock()
	return camFauxglMatrix, camPos
}

func (rm *renderer3mesh) depthBuffer() []float64 {
	return rm.lastContext.DepthBuffer
}

func (rm *renderer3mesh) renderBoundingBox(bb sdf.Box3, camFauxglMatrix fauxgl.Matrix, color color.Color) *image.NRGBA {
	mesh := fauxgl.NewCubeOutlineForBox(fauxgl.Box{
		Min: fauxgl.Vector{X: bb.Min.X, Y: bb.Min.Y, Z: -bb.Min.Z}, // FIXME: Swap back Z when camera is fixed
		Max: fauxgl.Vector{X: bb.Max.X, Y: bb.Max.Y, Z: -bb.Max.Z},
	})

	// Render the cube as a wireframe
	shader := fauxgl.NewSolidColorShader(camFauxglMatrix, fauxgl.MakeColor(color))
	rm.lastContext.Shader = shader
	rm.lastContext.Wireframe = true
	rm.lastContext.DrawMesh(mesh)

	return rm.lastContext.Image().(*image.NRGBA)
}

func r3mConvertTriangle(tri *sdf.Triangle3) *fauxgl.Triangle {
	normal := tri.Normal()
	normalV := r3mToFauxglVector(normal)
	return &fauxgl.Triangle{
		V1: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[0]), Normal: normalV, Color: fauxgl.Gray(1)},
		V2: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[1]), Normal: normalV, Color: fauxgl.Gray(1)},
		V3: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[2]), Normal: normalV, Color: fauxgl.Gray(1)},
	}
}

func r3mToFauxglVector(normal v3.Vec) fauxgl.Vector {
	return fauxgl.Vector{X: normal.X, Y: normal.Y, Z: normal.Z}
}

// r3mNormalShader
type r3mNormalShader struct {
	Matrix fauxgl.Matrix
}

func (shader *r3mNormalShader) Vertex(v fauxgl.Vertex) fauxgl.Vertex {
	v.Output = shader.Matrix.MulPositionW(v.Position)
	return v
}

func (shader *r3mNormalShader) Fragment(v fauxgl.Vertex) fauxgl.Color {
	return fauxgl.MakeColor(color.RGBA{
		R: uint8(math.Abs(v.Normal.X) * 255),
		G: uint8(math.Abs(v.Normal.Y) * 255),
		B: uint8(math.Abs(v.Normal.Z) * 255),
		A: 255,
	})
}
