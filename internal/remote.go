package internal

import (
	"context"
	"errors"
	"github.com/barkimedes/go-deepcopy"
	"github.com/deadsy/sdfx/sdf"
	"image"
	"log"
	"net/rpc"
	"os"
	"sync"
	"time"
)

// RendererService is an internal struct that has to be exported for RPC.
// is the server counterpart to rendererClient.
// It provides remote access to a devRendererImpl.
type RendererService struct {
	impl                        DevRendererImpl
	prevRenderCancel            func()
	renderCtx                   context.Context
	stateLock, cachedRenderLock *sync.RWMutex
	renders                     chan *RemoteRenderResults
	done                        chan os.Signal
}

// NewDevRendererService see RendererService
func NewDevRendererService(impl DevRendererImpl, done chan os.Signal) *rpc.Server {
	server := rpc.NewServer()
	srv := RendererService{
		impl:             impl,
		prevRenderCancel: func() {},
		renderCtx:        context.Background(),
		renders:          make(chan *RemoteRenderResults),
		done:             done,
	}
	close(srv.renders) // Mark the previous render as finished
	err := server.Register(&srv)
	if err != nil {
		panic(err) // Shouldn't happen (only on bad implementation)
	}
	return server
}

// Dimensions is an internal method that has to be exported for RPC.
func (d *RendererService) Dimensions(_ int, out *int) error {
	*out = d.impl.Dimensions()
	return nil
}

// BoundingBox is an internal method that has to be exported for RPC.
func (d *RendererService) BoundingBox(_ sdf.Box3, out *sdf.Box3) error {
	*out = d.impl.BoundingBox()
	return nil
}

// ColorModes is an internal method that has to be exported for RPC.
func (d *RendererService) ColorModes(_ int, out *int) error {
	*out = d.impl.ColorModes()
	return nil
}

// RemoteRenderArgs is an internal struct that has to be exported for RPC.
//goland:noinspection GoDeprecation
type RemoteRenderArgs struct {
	RenderSize sdf.V2i
	State      *RendererState
}

// RemoteRenderResults is an internal struct that has to be exported for RPC.
//goland:noinspection GoDeprecation
type RemoteRenderResults struct {
	IsPartial   bool
	RenderedImg *image.RGBA
	NewState    *RendererState
}

// RenderStart is an internal method that has to be exported for RPC.
// RenderStart starts a new render (cancelling the previous one)
func (d *RendererService) RenderStart(args RemoteRenderArgs, _ *int) error {
	d.prevRenderCancel() // Cancel previous render always (no concurrent renderings, although each rendering is parallel by itself)
	var newCtx context.Context
	newCtx, d.prevRenderCancel = context.WithCancel(context.Background())
loop: // Wait for previous renders to be properly completed/cancelled before continuing
	for {
		select {
		case <-newCtx.Done(): // End before started
			return newCtx.Err()
		case _, ok := <-d.renders:
			if !ok {
				break loop
			}
		}
	}
	d.stateLock = &sync.RWMutex{}
	d.cachedRenderLock = &sync.RWMutex{}
	d.cachedRenderLock.Lock()
	d.renderCtx = newCtx
	d.renders = make(chan *RemoteRenderResults)
	d.cachedRenderLock.Unlock()
	partialRenders := make(chan *image.RGBA)
	partialRendersFinish := make(chan struct{})
	go func() { // Start processing partial renders as requested (will silently drop it if not requested)
	loop:
		for partialRender := range partialRenders {
			select {
			case <-d.renderCtx.Done():
				log.Println("[DevRenderer] partialRender cancel")
				break loop
			case d.renders <- &RemoteRenderResults{
				IsPartial:   true,
				RenderedImg: partialRender,
				NewState:    args.State,
			}:
			default:
			}
		}
		close(partialRendersFinish)
	}()
	go func() { // spawn the blocking render in a different goroutine
		fullRender := image.NewRGBA(image.Rect(0, 0, args.RenderSize[0], args.RenderSize[1]))
		err := d.impl.Render(&RenderArgs{
			Ctx:              d.renderCtx,
			State:            args.State,
			StateLock:        d.stateLock,
			CachedRenderLock: d.cachedRenderLock,
			PartialRenders:   partialRenders,
			FullRender:       fullRender,
		})
		if err != nil {
			log.Println("[DevRenderer] RendererService.Render error:", err)
		}
		<-partialRendersFinish // Make sure all partial renders are sent before the full render
		if err == nil {        // Now we can send the full render
			select {
			case d.renders <- &RemoteRenderResults{
				IsPartial:   false,
				RenderedImg: fullRender,
				NewState:    args.State,
			}:
			case <-d.renderCtx.Done():
			}
		}
		close(d.renders)
	}()
	return nil
}

var errNoRenderRunning = errors.New("no render currently running")

// RenderGet is an internal struct that has to be exported for RPC.
// RenderGet gets the next partial or full render available (partial renders might be lost if not called, but not the full render).
// It will return an error if no render is running (or it was cancelled before returning the next result)
func (d *RendererService) RenderGet(_ int, out *RemoteRenderResults) error {
	//d.renderMu.Lock()
	//defer d.renderMu.Unlock()
	select {
	case read, ok := <-d.renders:
		if !ok {
			return errNoRenderRunning
		}
		out.IsPartial = read.IsPartial
		d.cachedRenderLock.RLock() // Need to perform a copy of the image to avoid races with the encoder task
		out.RenderedImg = image.NewRGBA(read.RenderedImg.Rect)
		copy(out.RenderedImg.Pix, read.RenderedImg.Pix)
		d.cachedRenderLock.RUnlock()
		d.stateLock.RLock()
		out.NewState = deepcopy.MustAnything(read.NewState).(*RendererState)
		d.stateLock.RUnlock()
		return nil
	case <-d.renderCtx.Done():
		return errNoRenderRunning // It was cancelled after get was called
	}
}

// RenderCancel is an internal struct that has to be exported for RPC.
// RenderCancel cancels the current rendering. It will always succeed with no error.
func (d *RendererService) RenderCancel(_ int, _ *int) error {
	//d.renderMu.Lock()
	//defer d.renderMu.Unlock()
	d.prevRenderCancel() // Cancel previous render
	return nil
}

// Shutdown is an internal struct that has to be exported for RPC.
// Shutdown sends a signal on the configured channel (with a timeout)
func (d *RendererService) Shutdown(t time.Duration, _ *int) error {
	select {
	case d.done <- os.Kill:
		return nil
	case <-time.After(t):
		return errors.New("shutdown timeout")
	}
}
