package ui

import (
	"github.com/deadsy/sdfx/vec/v2i"
	"github.com/hajimehoshi/ebiten"
)

// rendererEbitenGame hides the private ebiten implementation while behaving like a *Renderer internally
type rendererEbitenGame struct {
	*Renderer
}

func (r rendererEbitenGame) Update(_ *ebiten.Image) error {
	var err error
	r.cachedRenderLock.RLock()
	firstFrame := r.cachedRender == nil
	if firstFrame { // This always runs before the first frame
		r.cachedRender, err = ebiten.NewImage(1, 1, ebiten.FilterDefault)
		if err != nil {
			return err
		}
		r.cachedPartialRender = r.cachedRender
	}
	r.cachedRenderLock.RUnlock()
	r.onUpdateInputs()
	return nil
}

func (r rendererEbitenGame) Draw(screen *ebiten.Image) {
	r.drawSDF(screen)
	r.drawUI(screen)
}

func (r rendererEbitenGame) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	r.cachedRenderLock.RLock()
	firstFrame := r.cachedRender == nil
	r.cachedRenderLock.RUnlock()
	r.implStateLock.RLock()
	defer r.implStateLock.RUnlock()
	if !firstFrame { // Layout is called before Update(), but don't render in this case
		newScreenSize := v2i.Vec{X: outsideWidth, Y: outsideHeight}
		if r.screenSize != newScreenSize {
			r.screenSize = newScreenSize
			r.rerender()
		}
	}
	return outsideWidth, outsideHeight // Use all available pixels, no re-scaling (unless ResInv is modified)
}
