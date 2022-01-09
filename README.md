# SDFX-UI

SDFX-UI is a SDF (2D and 3D) IDE-independent renderer intended for fast development iterations that renders directly to a window. It integrates with your code as a library, meaning that you define your surface and then call the method that starts this UI.

## Demo
Configuring the Renderer is as simple as [creating your signed distance function](https://github.com/deadsy/sdfx) and calling:
```go
ui.NewRenderer(anySDF).Run()
```
The package will listen for code updates and display the new surfaces automatically.

`NewRenderer` also accepts several optional configuration parameters to customize the behavior (`ui.Opt*`).

https://user-images.githubusercontent.com/4929005/148616538-1251b1ed-0ae4-40a2-bbf0-c1b930580882.mp4

See examples/spiral/ for the complete code of this demo.

## How does it work?

The first time you run the code, it starts the renderer process. It also starts listening for code changes. When a code change is detected, the app is recompiled by the renderer (taking advantage of go's fast compilation times) and quickly renders the new surface to the same window (with the same camera position and other settings).

The SDF2 renderer shows the value of the SDF on each pixel using a grayscale: where bright pixels indicate outside the object and darker pixels are inside. The camera can be moved and scaled (using the mouse), rendering only the interesting part of the SDF.

SDF3s are raycasted from a perspective arc-ball camera that can be rotated around a pivot point, move its pivot and move closer or farther away from the pivot (using Blender-like mouse controls). Note that only the shown surface is actually rendered thanks to raycasting from the camera. This also means that the resulting surface can be much more detailed (depending on chosen resolution) than the triangle meshes generated by standard mesh generators.

It uses [ebiten](https://github.com/hajimehoshi/ebiten) for rendering, which is cross-platform, so it could also be used to showcase a surface (without automatic updates) creating an application for desktop, web, mobile or Nintendo Switch™.

## Browser and mobile demos
They use the same code, see compilation intructions at examples/spiral/main.go.

Note that mobile only works with mouse and keyboard for now (happy to receive pull requests).

![Screenshot_20220107_234547](https://user-images.githubusercontent.com/4929005/148616915-0cbd1126-7657-4fa6-b51e-2bd7b1a0e6a2.png)
![Screenshot_20220107-234815220](https://user-images.githubusercontent.com/4929005/148617130-b67ca779-28f0-4eda-873e-a8bb8dec5c72.jpg)
