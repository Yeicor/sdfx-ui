package ui

import (
	"github.com/Yeicor/sdfx-ui/internal"
	"github.com/barkimedes/go-deepcopy"
	"github.com/deadsy/sdfx/sdf"
	"log"
	"net/rpc"
	"time"
)

// rendererClient implements DevRendererImpl by calling a remote implementation (using Go's net/rpc)
type rendererClient struct {
	cl *rpc.Client
}

// newDevRendererClient see rendererClient
func newDevRendererClient(client *rpc.Client) internal.DevRendererImpl {
	return &rendererClient{cl: client}
}

func (d *rendererClient) Dimensions() int {
	var out int
	err := d.cl.Call("RendererService.Dimensions", &out, &out)
	if err != nil {
		log.Println("[DevRenderer] Error on remote call (RendererService.Dimensions):", err)
	}
	return out
}

func (d *rendererClient) BoundingBox() sdf.Box3 {
	var out sdf.Box3
	err := d.cl.Call("RendererService.BoundingBox", &out, &out)
	if err != nil {
		log.Println("[DevRenderer] Error on remote call (RendererService.BoundingBox):", err)
	}
	return out
}

func (d *rendererClient) ColorModes() int {
	var out int
	err := d.cl.Call("RendererService.ColorModes", &out, &out)
	if err != nil {
		log.Println("[DevRenderer] Error on remote call (RendererService.ColorModes):", err)
	}
	return out
}

func (d *rendererClient) Render(args *internal.RenderArgs) error {
	fullRenderSize := args.FullRender.Bounds().Size()
	args.StateLock.RLock() // Clone the state to avoid locking while the rendering is happening
	argsRemote := &internal.RemoteRenderArgs{
		RenderSize: sdf.V2i{fullRenderSize.X, fullRenderSize.Y},
		State:      deepcopy.MustAnything(args.State).(*internal.RendererState),
	}
	args.StateLock.RUnlock()
	var ignoreMe int
	err := d.cl.Call("RendererService.RenderStart", argsRemote, &ignoreMe)
	if err != nil {
		return err
	}
	for {
		var res internal.RemoteRenderResults
		err = d.cl.Call("RendererService.RenderGet", ignoreMe, &res)
		if err != nil {
			return err
		}
		select {
		case <-args.Ctx.Done(): // Cancel remote renderer also
			err = d.cl.Call("RendererService.RenderCancel", ignoreMe, &ignoreMe)
			if err != nil {
				log.Println("[DevRenderer] Error on remote call (RendererService.RenderCancel):", err)
			}
			return args.Ctx.Err()
		default:
		}
		if res.NewState != nil {
			args.StateLock.Lock() // Clone back the new state to avoid locking while the rendering is happening
			*args.State = *res.NewState
			args.StateLock.Unlock()
		}
		if res.IsPartial {
			if args.PartialRenders != nil {
				args.PartialRenders <- res.RenderedImg
			}
		} else { // Final render
			if args.PartialRenders != nil {
				close(args.PartialRenders)
			}
			args.CachedRenderLock.Lock()
			*args.FullRender = *res.RenderedImg
			args.CachedRenderLock.Unlock()
			break
		}
	}
	return err
}

func (d *rendererClient) Shutdown(timeout time.Duration) error {
	var out int
	return d.cl.Call("RendererService.Shutdown", &timeout, &out)
}

//goland:noinspection GoDeprecation

//goland:noinspection GoDeprecation
