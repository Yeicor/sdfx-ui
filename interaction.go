package ui

import (
	"context"
	"fmt"
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/barkimedes/go-deepcopy"
	"github.com/deadsy/sdfx/sdf"
	"github.com/hajimehoshi/ebiten"
	"github.com/hajimehoshi/ebiten/inpututil"
	"github.com/hajimehoshi/ebiten/text"
	"image/color"
	"math"
	"strconv"
	"time"
)

// onUpdateInputs handles inputs
func (r *Renderer) onUpdateInputs() {
	r.implLock.RLock()
	defer r.implLock.RUnlock()
	r.onUpdateInputsCommon()
	// SDF2/SDF3-SPECIFIC CONTROLS
	r.implStateLock.RLock()
	implDimCache := r.implDimCache
	r.implStateLock.RUnlock()
	switch implDimCache {
	case 2:
		r.onUpdateInputsSDF2()
	case 3:
		r.onUpdateInputsSDF3()
	default:
		panic("RendererState.onUpdateInputs not implemented for " + strconv.Itoa(r.implDimCache) + " dimensions")
	}
}

func (r *Renderer) onUpdateInputsCommon() {
	// SHARED CONTROLS
	if inpututil.IsKeyJustPressed(ebiten.KeyKPAdd) {
		r.implStateLock.Lock()
		r.implState.ResInv /= 2
		if r.implState.ResInv < 1 {
			r.implState.ResInv = 1
		}
		r.implStateLock.Unlock()
		r.rerender()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyKPSubtract) {
		r.implStateLock.Lock()
		r.implState.ResInv *= 2
		if r.implState.ResInv > 64 {
			r.implState.ResInv = 64
		}
		r.implStateLock.Unlock()
		r.rerender()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyB) {
		r.implStateLock.Lock()
		r.implState.DrawBbs = !r.implState.DrawBbs
		r.implStateLock.Unlock()
		r.rerender()
	}
	// Color
	if inpututil.IsKeyJustPressed(ebiten.KeyC) {
		r.implStateLock.Lock()
		r.implState.ColorMode = (r.implState.ColorMode + 1) % r.impl.ColorModes()
		r.implStateLock.Unlock()
		r.rerender()
	}
	if r.smoothCamera {
		r.implStateLock.RLock()
		if r.translateFrom[0] != math.MaxInt {
			r.rerenderOpt(false) // Trigger mid-movement rerender
		}
		r.implStateLock.RUnlock()
	}
}

func (r *Renderer) onUpdateInputsSDF2() {
	// Zooming
	_, wheelUpDown := ebiten.Wheel()
	if wheelUpDown != 0 {
		r.implStateLock.Lock()
		scale := 1 - wheelUpDown*r.implState.Bb.Size().Length2()*0.02   // Scale depending on current scale
		scale = math.Max(1/r.zoomFactor, math.Min(r.zoomFactor, scale)) // Apply zoom limits
		r.implState.Bb = r.implState.Bb.ScaleAboutCenter(scale)
		r.implStateLock.Unlock()
		r.rerender()
	}
	// Translation
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) || len(inpututil.JustPressedTouchIDs()) > 0 {
		// Save the cursor's position for previsualization and applying the final translation
		cx, cy := getCursor()
		r.implStateLock.Lock()
		if r.translateFrom[0] == math.MaxInt { // Only if not already moving...
			r.translateFrom = sdf.V2i{cx, cy}
		}
		r.implStateLock.Unlock()
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonMiddle) || inpututil.IsTouchJustReleased(0) {
		// Actually apply the translation and force a rerender
		cx, cy := getCursor()
		r.implStateLock.Lock()
		if r.translateFrom[0] != math.MaxInt { // Only if already moving...
			r.implState = r.apply2DCameraMoveTo(cx, cy)
			// Keep displacement until rerender is complete (avoid jump) using callback below + extra variable set here
			r.translateFromStop = sdf.V2i{cx, cy}
		}
		if r.smoothCamera {
			r.translateFrom = sdf.V2i{math.MaxInt, math.MaxInt}
		}
		r.implStateLock.Unlock()
		r.rerender(func(err error) {
			r.implStateLock.Lock()
			if !r.smoothCamera {
				r.translateFrom = sdf.V2i{math.MaxInt, math.MaxInt}
			}
			r.translateFromStop = sdf.V2i{math.MaxInt, math.MaxInt}
			r.implStateLock.Unlock()
		})
	}
	// Reset camera transform (100% of surface)
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		r.implStateLock.Lock()
		r.implState.Bb = toBox2(r.impl.BoundingBox()) // 100% zoom (impl2 will fix aspect ratio)
		r.implStateLock.Unlock()
		r.rerender()
	}
}

func (r *Renderer) apply2DCameraMoveTo(cx int, cy int) *internal.RendererState {
	newVal := deepcopy.MustAnything(r.implState).(*internal.RendererState)
	newVal.Bb = r.implState.Bb.Translate(
		r.translateFrom.ToV2().Sub(sdf.V2i{cx, cy}.ToV2()).Mul(sdf.V2{X: 1, Y: -1}). // Invert Y
												Div(r.screenSize.ToV2()).Mul(r.implState.Bb.Size()))
	return newVal
}

func getCursor() (int, int) {
	cx, cy := ebiten.CursorPosition()
	if tX, tY := ebiten.TouchPosition(0); tX != 0 && tY != 0 { // Override cursor with touch if available
		cx, cy = tX, tY // FIXME: Probably 0 does not exist anymore
	}
	return cx, cy
}

func (r *Renderer) onUpdateInputsSDF3() {
	// Zooming
	_, wheelUpDown := ebiten.Wheel()
	if wheelUpDown != 0 {
		r.implStateLock.Lock()
		scale := 1 - wheelUpDown*100
		scale = math.Max(1/r.zoomFactor, math.Min(r.zoomFactor, scale)) // Apply zoom limits
		r.implState.CamDist *= scale
		r.implStateLock.Unlock()
		r.rerender()
	}
	r.onUpdateInputsSDF3RotTrans()
	// Reset camera transform
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		r.implStateLock.Lock()
		resetCam3(r.implState, r)
		r.implStateLock.Unlock()
		r.rerender()
	}
}

func (r *Renderer) onUpdateInputsSDF3RotTrans() {
	// Rotation + Translation
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonMiddle) || len(inpututil.JustPressedTouchIDs()) > 0 {
		// Save the cursor's position for previsualization and applying the final translation
		cx, cy := getCursor()
		r.implStateLock.Lock()
		if r.translateFrom[0] == math.MaxInt { // Only if not already moving...
			r.translateFrom = sdf.V2i{cx, cy}
		}
		r.implStateLock.Unlock()
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonMiddle) || inpututil.IsTouchJustReleased(0) {
		// Actually apply the translation and force a rerender
		cx, cy := getCursor()
		r.implStateLock.Lock()
		if r.translateFrom[0] != math.MaxInt { // Only if already moving...
			r.implState = r.apply3DCameraMoveTo(cx, cy)
			// Keep displacement until rerender is complete (avoid jump) using callback below + extra variable set here
			r.translateFromStop = sdf.V2i{cx, cy}
		}
		if r.smoothCamera {
			r.translateFrom = sdf.V2i{math.MaxInt, math.MaxInt}
		}
		r.implStateLock.Unlock()
		r.rerender(func(err error) {
			r.implStateLock.Lock()
			if !r.smoothCamera {
				r.translateFrom = sdf.V2i{math.MaxInt, math.MaxInt}
			}
			r.translateFromStop = sdf.V2i{math.MaxInt, math.MaxInt}
			r.implStateLock.Unlock()
		})
	}
}

func (r *Renderer) apply3DCameraMoveTo(cx int, cy int) *internal.RendererState {
	newVal := deepcopy.MustAnything(r.implState).(*internal.RendererState)
	delta := sdf.V2i{cx, cy}.ToV2().Sub(r.translateFrom.ToV2())
	if ebiten.IsKeyPressed(ebiten.KeyShift) { // Translation
		// Move on the plane perpendicular to the camera's direction
		camViewMatrix := cam3MatrixNoTranslation(r.implState)
		camPos := r.implState.CamCenter.Add(camViewMatrix.MulPosition(sdf.V3{Y: -r.implState.CamDist}))
		camDir := r.implState.CamCenter.Sub(camPos).Normalize()
		planeZero := r.implState.CamCenter
		planeRight := sdf.V3{Z: 1}.Cross(camDir).Normalize()
		planeUp := camDir.Cross(planeRight).Normalize()
		newPos := planeZero. // TODO: Proper projection on plane delta computation
					Add(planeRight.MulScalar(delta.X * r.implState.CamDist / 200)).
					Add(planeUp.MulScalar(delta.Y * r.implState.CamDist / 200))
		newVal.CamCenter = newPos
		//log.Println("New camera pivot (center", r.implState.CamCenter, ")")
	} else { // Rotation
		newVal.CamYaw -= delta.X / 100 // TODO: Proper delta computation
		if newVal.CamYaw < -math.Pi {
			newVal.CamYaw += 2 * math.Pi // Limits (wrap around)
		} else if newVal.CamYaw > math.Pi {
			newVal.CamYaw -= 2 * math.Pi // Limits (wrap around)
		}
		newVal.CamPitch -= delta.Y / 100
		newVal.CamPitch = math.Max(-(math.Pi/2 - 1e-5), math.Min(math.Pi/2-1e-5, newVal.CamPitch))
		//log.Println("New camera rotation (pitch", r.implState.CamPitch, "yaw", r.implState.CamYaw, ")")
	}
	return newVal
}

// ControlsText returns the help text
func (r *Renderer) drawUI(screen *ebiten.Image) {
	// Notify when rendering
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancelFunc()
	if r.renderingLock.RTryLock(ctx) {
		r.renderingLock.RUnlock()
	} else {
		drawDefaultTextWithShadow(screen, "Rendering...", 5, 5+12, color.RGBA{R: 255, A: 255})
	}

	// Draw current state and controls
	r.implStateLock.RLock()
	defer r.implStateLock.RUnlock()
	msgFmt := "TPS: %0.2f/%d\nResolution: %.2f [+/-]\nColor: %d [C]\nBoxes: %t [B]\nReset camera [R]"
	msgValues := []interface{}{ebiten.CurrentTPS(), ebiten.MaxTPS(), 1 / float64(r.implState.ResInv), r.implState.ColorMode, r.implState.DrawBbs}
	switch r.implDimCache {
	case 2:
		msgFmt = "SDF2 Renderer\n=============\n" + msgFmt + "\nTranslate cam [MiddleMouse]\nZoom cam [MouseWheel]"
	case 3:
		msgFmt = "SDF3 Renderer\n=============\n" + msgFmt + "\nRotate cam [MiddleMouse]\nTranslate cam [Shift+MiddleMouse]\nZoom cam [MouseWheel]"
	}
	msg := fmt.Sprintf(msgFmt, msgValues...)
	boundString := text.BoundString(defaultFont, msg)
	drawDefaultTextWithShadow(screen, msg, 5, r.screenSize[1]-boundString.Size().Y+10, color.RGBA{G: 255, A: 255})
}
