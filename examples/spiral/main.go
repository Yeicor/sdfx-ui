package main

import (
	dev "github.com/Yeicor/sdfx-ui"
	"github.com/deadsy/sdfx/sdf"
	"github.com/hajimehoshi/ebiten"
	"log"
)

func spiralSdf() (s interface{}, err error) {
	s, err = sdf.ArcSpiral2D(1.0, 20.0, 0.25*sdf.Pi, 8*sdf.Tau, 1.0)
	if err != nil {
		return nil, err
	}

	//c, err := sdf.Circle2D(22.)
	//if err != nil {
	//	return nil, err
	//}
	//s = sdf.Union2D(s.(sdf.SDF2), c)

	//c2, err := sdf.Circle2D(20.)
	//if err != nil {
	//	return nil, err
	//}
	//c2 = sdf.Transform2D(c2, sdf.Translate2d(sdf.V2{X: 0}))
	//s = sdf.Difference2D(s.(sdf.SDF2), c2)

	//WARNING: Text is slow to render (especially with -race flag)
	//f, err := sdf.LoadFont("../text/cmr10.ttf")
	//if err != nil {
	//	log.Fatalf("can't read font file %s\n", err)
	//}
	//t, err := sdf.TextSDF2(f, sdf.NewText("Spiral"), 10)
	//if err != nil {
	//	return nil, err
	//}
	//s = sdf.Difference2D(s.(sdf.SDF2), t)

	//s = sdf.Extrude3D(s.(sdf.SDF2), 4)
	s, _ = sdf.ExtrudeRounded3D(s.(sdf.SDF2), 4, 0.25)
	//s, _ = sdf.RevolveTheta3D(s.(sdf.SDF2), math.Pi/2)

	//box3, _ := sdf.Box3D(sdf.V3{X: 20, Y: 10, Z: 5}, 0.2)
	//box3 = sdf.Transform3D(box3, sdf.Translate3d(sdf.V3{Z: 15}))
	//s = sdf.Union3D(s.(sdf.SDF3), box3)

	return s, err
}

// DESKTOP: go run .
// BROWSER: $(go env GOPATH)/bin/wasmserve .
// MOBILE: $(go env GOPATH)/bin/gomobile build -v -target=android .
func main() {
	s, err := spiralSdf()
	if err != nil {
		log.Fatalf("error: %s\n", err)
	}

	// Rendering configuration boilerplate
	ebiten.SetWindowTitle("SDFX spiral dev renderer demo")
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowResizable(true)
	//ebiten.SetWindowPosition(3000, 0)
	//ebiten.SetWindowSize(1920, 1040)

	//// Profiling boilerplate
	//defer func() {
	//	//cmd := exec.Command("go", "tool", "pprof", "cpu.pprof")
	//	cmd := exec.Command("go", "tool", "trace", "trace.out")
	//	cmd.Stdin = os.Stdin
	//	cmd.Stdout = os.Stdout
	//	cmd.Stderr = os.Stderr
	//	err = cmd.Run()
	//	if err != nil {
	//		panic(err)
	//	}
	//}()
	////defer profile.Start(profile.ProfilePath(".")).Stop()
	//defer profile.Start(profile.TraceProfile, profile.ProfilePath(".")).Stop()

	// Actual rendering loop
	err = dev.NewRenderer(s,
		dev.OptMWatchFiles([]string{"main.go"}), // Default of "." also works, but it triggers too often if generating a profile
	).Run()
	if err != nil {
		panic(err)
	}
}
