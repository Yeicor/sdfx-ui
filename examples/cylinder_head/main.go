//-----------------------------------------------------------------------------
/*

SOURCE: https://github.com/deadsy/sdfx/tree/master/examples/cylinder_head

Wallaby Cylinder Head

No draft version for 3d printing and lost-PLA investment casting.

*/
//-----------------------------------------------------------------------------

package main

import (
	ui "github.com/Yeicor/sdfx-ui"
	"github.com/deadsy/sdfx/render"
	"github.com/deadsy/sdfx/sdf"
	v2 "github.com/deadsy/sdfx/vec/v2"
	v3 "github.com/deadsy/sdfx/vec/v3"
	"github.com/hajimehoshi/ebiten"
	"math"
)

//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------
// scaling

const desiredScale = 1.25
const alShrink = 1.0 / 0.99   // ~1%
const plaShrink = 1.0 / 0.998 //~0.2%

// dimension scaling
func dim(x float64) float64 {
	return x * desiredScale * sdf.MillimetresPerInch * alShrink * plaShrink
}

var generalRound = dim(0.1)

//-----------------------------------------------------------------------------
// exhaust bosses

var ebSideRadius = dim(5.0 / 32.0)
var ebMainRadius = dim(5.0 / 16.0)
var ebHoleRadius = dim(3.0 / 16.0)
var ebC2cDistance = dim(13.0 / 16.0)
var ebDistance = ebC2cDistance / 2.0

var ebXOffset = 0.5*(headLength+ebHeight) - ebHeight0
var ebYOffset = (headWidth / 2.0) - ebDistance - ebSideRadius
var ebZOffset = dim(1.0 / 16.0)

var ebHeight0 = dim(1.0 / 16.0)
var ebHeight1 = dim(1.0 / 8.0)
var ebHeight = ebHeight0 + ebHeight1

func exhaustBoss(mode string, xOfs float64) sdf.SDF3 {

	var s0 sdf.SDF2

	if mode == "body" {
		s0 = sdf.NewFlange1(ebDistance, ebMainRadius, ebSideRadius)
	} else if mode == "hole" {
		s0, _ = sdf.Circle2D(ebHoleRadius)
	} else {
		panic("bad mode")
	}

	s1 := sdf.Extrude3D(s0, ebHeight)
	m := sdf.RotateZ(sdf.DtoR(90))
	m = sdf.RotateY(sdf.DtoR(90)).Mul(m)
	m = sdf.Translate3d(v3.Vec{X: xOfs, Y: ebYOffset, Z: ebZOffset}).Mul(m)
	s1 = sdf.Transform3D(s1, m)
	return s1
}

func exhaustBosses(mode string) sdf.SDF3 {
	return sdf.Union3D(exhaustBoss(mode, ebXOffset), exhaustBoss(mode, -ebXOffset))
}

//-----------------------------------------------------------------------------
// spark plug bosses

var sp2spDistance = dim(1.0 + (5.0 / 8.0))
var spTheta = sdf.DtoR(30)

var spBossR1 = dim(21.0 / 64.0)
var spBossR2 = dim(15.0 / 32.0)
var spBossH1 = dim(0.79)
var spBossH2 = dim(0.94)
var spBossH3 = dim(2)

var spHoleD = dim(21.0 / 64.0)
var spHoleR = spHoleD / 2.0
var spHoleH = dim(1.0)

var spCbH1 = dim(1.0)
var spCbH2 = dim(2.0)
var spCbR = dim(5.0 / 16.0)

var spHyp = spHoleH + spCbR*math.Tan(spTheta)
var spYOfs = spHyp*math.Cos(spTheta) - headWidth/2
var spZOfs = -spHyp * math.Sin(spTheta)

func sparkplug(mode string, xOfs float64) sdf.SDF3 {
	var vlist []v2.Vec
	if mode == "boss" {
		boss := sdf.NewPolygon()
		boss.Add(0, 0)
		boss.Add(spBossR1, 0)
		boss.Add(spBossR1, spBossH1).Smooth(spBossR1*0.3, 3)
		boss.Add(spBossR2, spBossH2).Smooth(spBossR2*0.3, 3)
		boss.Add(spBossR2, spBossH3)
		boss.Add(0, spBossH3)
		vlist = boss.Vertices()
	} else if mode == "hole" {
		vlist = []v2.Vec{
			{0, 0},
			{spHoleR, 0},
			{spHoleR, spHoleH},
			{0, spHoleH},
		}
	} else if mode == "counterbore" {
		p := sdf.NewPolygon()
		p.Add(0, spCbH1)
		p.Add(spCbR, spCbH1).Smooth(spCbR/6.0, 3)
		p.Add(spCbR, spCbH2)
		p.Add(0, spCbH2)
		vlist = p.Vertices()
	} else {
		panic("bad mode")
	}
	s0, _ := sdf.Polygon2D(vlist)
	s, _ := sdf.Revolve3D(s0)
	m := sdf.RotateX(sdf.Pi/2 - spTheta)
	m = sdf.Translate3d(v3.Vec{X: xOfs, Y: spYOfs, Z: spZOfs}).Mul(m)
	s = sdf.Transform3D(s, m)
	return s
}

func sparkplugs(mode string) sdf.SDF3 {
	xOfs := 0.5 * sp2spDistance
	return sdf.Union3D(sparkplug(mode, xOfs), sparkplug(mode, -xOfs))
}

//-----------------------------------------------------------------------------
// valve bosses

var valveDiameter = dim(1.0 / 4.0)
var valveRadius = valveDiameter / 2.0
var valveYOffset = dim(1.0 / 8.0)
var valveWall = dim(5.0 / 32.0)
var v2vDistance = dim(1.0 / 2.0)
var valveDraft = sdf.DtoR(5)

func valve(d float64, mode string) sdf.SDF3 {

	var s sdf.SDF3
	h := headHeight - cylinderHeight

	if mode == "boss" {
		delta := h * math.Tan(valveDraft)
		r1 := valveRadius + valveWall
		r0 := r1 + delta
		s, _ = sdf.Cone3D(h, r0, r1, 0)
	} else if mode == "hole" {
		s, _ = sdf.Cylinder3D(h, valveRadius, 0)
	} else {
		panic("bad mode")
	}

	zOfs := cylinderHeight / 2
	return sdf.Transform3D(s, sdf.Translate3d(v3.Vec{X: d, Y: valveYOffset, Z: zOfs}))
}

func valveSet(d float64, mode string) sdf.SDF3 {
	delta := v2vDistance / 2
	s := sdf.Union3D(valve(-delta, mode), valve(delta, mode))
	s.(*sdf.UnionSDF3).SetMin(sdf.PolyMin(generalRound))
	return sdf.Transform3D(s, sdf.Translate3d(v3.Vec{X: d}))
}

func valveSets(mode string) sdf.SDF3 {
	delta := c2cDistance / 2
	return sdf.Union3D(valveSet(-delta, mode), valveSet(delta, mode))
}

//-----------------------------------------------------------------------------
// cylinder domes (or full base)

var cylinderHeight = dim(3.0 / 16.0)
var cylinderDiameter = dim(1.0 + (1.0 / 8.0))
var cylinderWall = dim(1.0 / 4.0)
var cylinderRadius = cylinderDiameter / 2.0

var domeRadius = cylinderWall + cylinderRadius
var domeHeight = cylinderWall + cylinderHeight

var c2cDistance = dim(1.0 + (3.0 / 8.0))

func cylinderHead(d float64, mode string) sdf.SDF3 {
	var s sdf.SDF3

	if mode == "dome" {
		zOfs := (headHeight - domeHeight) / 2
		extraZ := generalRound * 2
		s, _ = sdf.Cylinder3D(domeHeight+extraZ, domeRadius, generalRound)
		s = sdf.Transform3D(s, sdf.Translate3d(v3.Vec{X: d, Z: -zOfs - extraZ}))
	} else if mode == "chamber" {
		zOfs := (headHeight - cylinderHeight) / 2
		s, _ = sdf.Cylinder3D(cylinderHeight, cylinderRadius, 0)
		s = sdf.Transform3D(s, sdf.Translate3d(v3.Vec{X: d, Z: -zOfs}))
	} else {
		panic("bad mode")
	}
	return s
}

func cylinderHeads(mode string) sdf.SDF3 {
	xOfs := c2cDistance / 2
	s := sdf.Union3D(cylinderHead(-xOfs, mode), cylinderHead(xOfs, mode))
	if mode == "dome" {
		s.(*sdf.UnionSDF3).SetMin(sdf.PolyMin(generalRound))
	}
	return s
}

//-----------------------------------------------------------------------------
// cylinder studs: location, bosses and holes

var studBossRadius = dim(3.0 / 16.0)
var studHoleDy = dim(11.0 / 16.0)
var studHoleDx0 = dim(7.0 / 16.0)
var studHoleDx1 = dim(1.066)

var studLocations = []v2.Vec{
	{studHoleDx0 + studHoleDx1, 0},
	{studHoleDx0 + studHoleDx1, studHoleDy},
	{studHoleDx0 + studHoleDx1, -studHoleDy},
	{studHoleDx0, studHoleDy},
	{studHoleDx0, -studHoleDy},
	{-studHoleDx0 - studHoleDx1, 0},
	{-studHoleDx0 - studHoleDx1, studHoleDy},
	{-studHoleDx0 - studHoleDx1, -studHoleDy},
	{-studHoleDx0, studHoleDy},
	{-studHoleDx0, -studHoleDy},
}

//-----------------------------------------------------------------------------
// head walls

var headLength = dim(4.30 / 1.25)
var headWidth = dim(2.33 / 1.25)
var headHeight = dim(7.0 / 8.0)
var headCornerRound = dim((5.0 / 32.0) / 1.25)
var headWallThickness = dim(0.154)

func headWallOuter2d() sdf.SDF2 {
	return sdf.Box2D(v2.Vec{X: headLength, Y: headWidth}, headCornerRound)
}

func headWallInner2d() sdf.SDF2 {
	l := headLength - (2 * headWallThickness)
	w := headWidth - (2 * headWallThickness)
	s0 := sdf.Box2D(v2.Vec{X: l, Y: w}, 0)
	c, _ := sdf.Circle2D(studBossRadius)
	s1 := sdf.Multi2D(c, studLocations)
	s := sdf.Difference2D(s0, s1)
	s.(*sdf.DifferenceSDF2).SetMax(sdf.PolyMax(generalRound))
	return s
}

func headEnvelope() sdf.SDF3 {
	s0 := sdf.Box2D(v2.Vec{X: headLength + 2*ebHeight1, Y: headWidth}, 0)
	return sdf.Extrude3D(s0, headHeight)
}

func headWall() sdf.SDF3 {
	s := headWallOuter2d()
	s = sdf.Difference2D(s, headWallInner2d())
	return sdf.Extrude3D(s, headHeight)
}

//-----------------------------------------------------------------------------
// manifolds

var manifoldRadius = dim(4.5 / 16.0)
var manifoldHoleRadius = dim(1.0 / 8.0)
var inletTheta = 30.2564
var exhaustTheta = 270.0 + 13.9736
var exhaustXOffset = (c2cDistance / 2) + (v2vDistance / 2)
var inletXOffset = (c2cDistance / 2) - (v2vDistance / 2)

func manifoldSet(r float64) sdf.SDF3 {

	h := dim(2)

	sEx, _ := sdf.Cylinder3D(h, r, 0)
	m := sdf.Translate3d(v3.Vec{Z: h / 2})
	m = sdf.RotateX(sdf.DtoR(-90)).Mul(m)
	m = sdf.RotateZ(sdf.DtoR(exhaustTheta)).Mul(m)
	m = sdf.Translate3d(v3.Vec{X: exhaustXOffset, Y: valveYOffset, Z: ebZOffset}).Mul(m)
	sEx = sdf.Transform3D(sEx, m)

	sIn, _ := sdf.Cylinder3D(h, r, 0)
	m = sdf.Translate3d(v3.Vec{Z: h / 2})
	m = sdf.RotateX(sdf.DtoR(-90)).Mul(m)
	m = sdf.RotateZ(sdf.DtoR(inletTheta)).Mul(m)
	m = sdf.Translate3d(v3.Vec{X: inletXOffset, Y: valveYOffset, Z: ebZOffset}).Mul(m)
	sIn = sdf.Transform3D(sIn, m)

	return sdf.Union3D(sEx, sIn)
}

func manifolds(mode string) sdf.SDF3 {
	var r float64
	if mode == "body" {
		r = manifoldRadius
	} else if mode == "hole" {
		r = manifoldHoleRadius
	} else {
		panic("bad mode")
	}
	s0 := manifoldSet(r)
	s1 := sdf.Transform3D(s0, sdf.MirrorYZ())
	s := sdf.Union3D(s0, s1)
	if mode == "body" {
		s.(*sdf.UnionSDF3).SetMin(sdf.PolyMin(generalRound))
	}
	return s
}

//-----------------------------------------------------------------------------

func allowances(_ sdf.SDF3) sdf.SDF3 {
	//eb0_2d := Slice2D(s, V3{eb_x_offset, 0, 0}, V3{1, 0, 0})
	//return Extrude3D(eb0_2d, 10.0)
	return nil
}

//-----------------------------------------------------------------------------

func additive() sdf.SDF3 {
	s := sdf.Union3D(
		headWall(),
		//head_base(),
		cylinderHeads("dome"),
		valveSets("boss"),
		sparkplugs("boss"),
		manifolds("body"),
		exhaustBosses("body"),
	)
	s.(*sdf.UnionSDF3).SetMin(sdf.PolyMin(generalRound))

	s = sdf.Difference3D(s, sparkplugs("counterbore"))

	// cleanup the blending artifacts on the outside
	s = sdf.Intersect3D(s, headEnvelope())

	//if casting == true {
	s = sdf.Union3D(s, allowances(s))
	//}

	return s
}

//-----------------------------------------------------------------------------

func subtractive() sdf.SDF3 {
	var s sdf.SDF3
	//if casting == false {
	//	s = sdf.Union3D(cylinder_heads("chamber"),
	//		head_stud_holes(),
	//		valve_sets("hole"),
	//		sparkplugs("hole"),
	//		manifolds("hole"),
	//		exhaust_bosses("hole"),
	//	)
	//}
	return s
}

//-----------------------------------------------------------------------------

func main() {
	s := sdf.Difference3D(additive(), subtractive())

	ebiten.SetWindowTitle("SDFX-UI cylinder head demo")
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetWindowResizable(true)

	// Actual rendering loop
	err := ui.NewRenderer(s,
		ui.OptMWatchFiles([]string{"main.go"}), // Default of "." also works, but it triggers too often if generating a profile
		ui.Opt3Mesh(render.NewMarchingCubesUniform(100), math.Pi/3),
		ui.OptMSmoothCamera(true),
	).Run()
	if err != nil {
		panic(err)
	}
}

//-----------------------------------------------------------------------------
