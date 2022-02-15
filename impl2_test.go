package ui

import (
	"context"
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/sdf"
	"image"
	"sync"
	"testing"
)

func BenchmarkDevRenderer2_Render(b *testing.B) {
	s, _ := sdf.ArcSpiral2D(1.0, 20.0, 0.25*sdf.Pi, 8*sdf.Tau, 1.0)
	impl := newDevRenderer2(s)
	b.ReportAllocs()
	state := RendererState{
		ResInv: 8,
		Bb:     s.BoundingBox(),
	}
	fullRender := image.NewRGBA(image.Rect(0, 0, 1920/state.ResInv, 1080/state.ResInv))
	lock1 := &sync.RWMutex{}
	lock2 := &sync.RWMutex{}
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		err := impl.Render(&internal.RenderArgs{Ctx: context.Background(), State: &state, StateLock: lock1, CachedRenderLock: lock2, FullRender: fullRender})
		if err != nil {
			b.Fatal(err)
		}
	}
}
