package dev

import (
	"github.com/deadsy/sdfx/render"
	"github.com/deadsy/sdfx/sdf"
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
func Opt3Mesh(meshGenerator render.Render3, meshCells int, smoothNormalsRadians float64) Option {
	return func(r *Renderer) {
		if r3, ok := r.impl.(*renderer3); ok {
			log.Println("[DevRenderer] Rendering 3D mesh...") // only performed once per compilation
			var triangles []*fauxgl.Triangle
			triChan := make(chan *render.Triangle3)
			go func() {
				meshGenerator.Render(r3.s, meshCells, triChan)
				close(triChan)
			}()
			for tri := range triChan {
				triangles = append(triangles, r3mConvertTriangle(tri))
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

func (rm *renderer3mesh) Render(r *renderer3, args *renderArgs) error {
	args.stateLock.Lock()
	bounds := args.fullRender.Bounds()
	boundsSize := sdf.V2i{bounds.Size().X, bounds.Size().Y}
	if rm.lastContext == nil || rm.lastContext.Width != boundsSize[0] || rm.lastContext.Height != boundsSize[1] {
		// Rebuild rendering context only when needed
		rm.lastContext = fauxgl.NewContext(boundsSize[0], boundsSize[1])
	} else {
		rm.lastContext.ClearDepthBuffer()
	}
	rm.lastContext.ClearColorBufferWith(fauxgl.MakeColor(r.backgroundColor))

	// Compute camera matrix and more (once per render)
	//args.state.CamYaw += math.Pi // HACK
	//args.state.CamCenter.X = -args.state.CamCenter.X
	//args.state.CamCenter.Y = -args.state.CamCenter.Y
	aspectRatio := float64(boundsSize[0]) / float64(boundsSize[1])
	camViewMatrix := args.state.Cam3MatrixNoTranslation()
	camPos := args.state.CamCenter.Add(camViewMatrix.MulPosition(sdf.V3{Y: -args.state.CamDist / 1.12 /* Adjust to other implementation*/}))
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
	camFauxglMatrix := fauxgl.LookAt(r3mToFauxglVector(camPos), r3mToFauxglVector(args.state.CamCenter), fauxgl.Vector{Z: 1}).
		Perspective(camFovY*180/math.Pi, aspectRatio, 1e-6, maxRay)
	//args.state.CamYaw -= math.Pi // HACK (restore)
	//args.state.CamCenter.X = -args.state.CamCenter.X
	//args.state.CamCenter.Y = -args.state.CamCenter.Y
	args.stateLock.Unlock()

	// Configure the shader (based on ColorMode)
	if args.state.ColorMode == 0 {
		// use builtin phong shader
		shader := fauxgl.NewPhongShader(camFauxglMatrix, r3mToFauxglVector(r.lightDir), r3mToFauxglVector(camPos))
		shader.ObjectColor = fauxgl.MakeColor(r.surfaceColor)
		rm.lastContext.Shader = shader
		rm.lastContext.Wireframe = false
	} else {
		// use normal based shader
		rm.lastContext.Shader = &r3mNormalShader{camFauxglMatrix}
		rm.lastContext.Wireframe = args.state.ColorMode == 2 // set to wireframe mode
	}
	// Perform the actual render
	rm.lastContext.DrawMesh(rm.mesh) // This is already multithread, no need to parallelize anymore
	img := rm.lastContext.Image()

	// Copy output full render (no partial renders supported)
	args.cachedRenderLock.Lock()
	copy(args.fullRender.Pix[args.fullRender.PixOffset(0, 0):], img.(*image.NRGBA).Pix[img.(*image.NRGBA).PixOffset(0, 0):])
	args.cachedRenderLock.Unlock()

	// TODO: Draw bounding boxes over the image

	if args.partialRenders != nil {
		close(args.partialRenders)
	}

	return nil
}

func r3mConvertTriangle(tri *render.Triangle3) *fauxgl.Triangle {
	normal := tri.Normal()
	normalV := r3mToFauxglVector(normal)
	return &fauxgl.Triangle{
		V1: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[0]), Normal: normalV, Color: fauxgl.Gray(1)},
		V2: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[1]), Normal: normalV, Color: fauxgl.Gray(1)},
		V3: fauxgl.Vertex{Position: r3mToFauxglVector(tri.V[2]), Normal: normalV, Color: fauxgl.Gray(1)},
	}
}

func r3mToFauxglVector(normal sdf.V3) fauxgl.Vector {
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
