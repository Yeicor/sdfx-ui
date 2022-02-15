package ui

import (
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/sdf"
	"math"
)

func (r *Renderer) newRendererState() *internal.RendererState {
	r.implLock.RLock()
	defer r.implLock.RUnlock()
	s := &internal.RendererState{
		ResInv: 4,
		Bb:     toBox2(r.impl.BoundingBox()), // 100% zoom (will fix aspect ratio later)
	}
	resetCam3(s, r)
	return s
}

func cam3MatrixNoTranslation(s *internal.RendererState) sdf.M44 {
	return sdf.RotateZ(s.CamYaw).Mul(sdf.RotateX(s.CamPitch))
}

func resetCam3(s *internal.RendererState, r *Renderer) {
	s.CamCenter = r.impl.BoundingBox().Center()
	s.CamDist = r.impl.BoundingBox().Size().Length() / 2
	s.CamPitch = -math.Pi / 4 // Look from 45º up
	s.CamYaw = -math.Pi / 4   // Look from 45º right
}
