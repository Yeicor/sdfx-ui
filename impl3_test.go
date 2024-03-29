package ui

import (
	"context"
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/deadsy/sdfx/sdf"
	v3 "github.com/deadsy/sdfx/vec/v3"
	"image"
	"math"
	"sync"
	"testing"
)

func BenchmarkDevRenderer3_Render(b *testing.B) {
	s, _ := sdf.ArcSpiral2D(1.0, 20.0, 0.25*sdf.Pi, 8*sdf.Tau, 1.0)
	s3, _ := sdf.ExtrudeRounded3D(s, 4, 1)
	impl := newDevRenderer3(s3)
	b.ReportAllocs()
	state := internal.RendererState{
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

func Test_collideRayBb(t *testing.T) {
	type args struct {
		origin v3.Vec
		dir    v3.Vec
		bb     sdf.Box3
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "Basic",
			args: args{
				origin: v3.Vec{Z: -2},
				dir:    v3.Vec{Z: 1},
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: 1,
		},
		{
			name: "Sideways",
			args: args{
				origin: v3.Vec{X: -2, Y: -2, Z: -2},
				dir:    v3.Vec{X: 1, Y: 1, Z: 1}.Normalize(),
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: v3.Vec{X: 1, Y: 1, Z: 1}.Length(),
		},
		{
			name: "Backwards",
			args: args{
				origin: v3.Vec{X: 2, Y: 2, Z: 2},
				dir:    v3.Vec{X: 1, Y: 1, Z: 1}.Normalize(),
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: -v3.Vec{X: 1, Y: 1, Z: 1}.Length(),
		},
		{
			name: "Inside",
			args: args{
				origin: v3.Vec{X: 0.1, Y: 0.1, Z: 0.1},
				dir:    v3.Vec{X: 1, Y: 1, Z: 1}.Normalize(),
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: v3.Vec{X: 0.9, Y: 0.9, Z: 0.9}.Length(),
		},
		{
			name: "Inside2",
			args: args{
				origin: v3.Vec{X: 0.1, Y: 0.1, Z: 0.1},
				dir:    v3.Vec{X: -1, Y: -1, Z: -1}.Normalize(),
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: v3.Vec{X: -1.1, Y: -1.1, Z: -1.1}.Length(),
		},
		{
			name: "No hit",
			args: args{
				origin: v3.Vec{X: 10, Y: 0, Z: 0},
				dir:    v3.Vec{X: 1, Y: 1, Z: 1}.Normalize(),
				bb: sdf.Box3{
					Min: v3.Vec{X: -1, Y: -1, Z: -1},
					Max: v3.Vec{X: 1, Y: 1, Z: 1},
				},
			},
			want: -15.588457268119893,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := collideRayBb(tt.args.origin, tt.args.dir, tt.args.bb); math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("collideRayBb() = %v, want %v", got, tt.want)
			}
		})
	}
}
