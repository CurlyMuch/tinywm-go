package main

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
)

func main () {
	values := []uint32{0, 0, 0}
	X, err := xgb.NewConn() // Run with env var example: DISPLAY=:0
	if err != nil {
		panic(err)
	}

	var win xproto.Window
	screen := xproto.Setup(X).DefaultScreen(X)
	root := screen.Root

	xproto.GrabKey(X, true, root, xproto.ModMask2, 0,
		xproto.GrabModeAsync, xproto.GrabModeAsync)

	xproto.GrabButton(X, false, root,
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync, xproto.GrabModeAsync, root, 0, 1, xproto.ModMask1)

	xproto.GrabButton(X, false, root,
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync, xproto.GrabModeAsync, root, 0, 3, xproto.ModMask1)

	X.Sync()

	for {
		ev, err := X.WaitForEvent()
		// Will be X error, apparently can be ignored
		if err != nil {
			fmt.Println(err)
			continue
		}
		// Handle connection closed
		if ev == nil && err == nil {
			break
		}

		switch v := ev.(type) {
		case xproto.ButtonPressEvent:
			fmt.Println("ButtonPressed")
			win = v.Child
			values[0] = xproto.StackModeAbove
			xproto.ConfigureWindow(X, win, xproto.ConfigWindowStackMode, values)
			geom, err := xproto.GetGeometry(X, xproto.Drawable(win)).Reply()
			if err != nil {
				fmt.Println(err)
				break
			}
			if v.Detail == 1 {
				values[2] = 1
				xproto.WarpPointer(X, 0, win, 0, 0, 0, 0, 1, 1)
			} else {
				values[2] = 3
				xproto.WarpPointer(X, 0, win, 0, 0 ,0 ,0,
					int16(geom.Width), int16(geom.Height))
			}
			xproto.GrabPointer(X, false, root, xproto.EventMaskButtonRelease |
				xproto.EventMaskButtonMotion | xproto.EventMaskPointerMotion,
				xproto.GrabModeAsync, xproto.GrabModeAsync, root, 0,
				xproto.TimeCurrentTime)
			X.Sync()
		case xproto.MotionNotifyEvent:
			action := values[2]
			if action == 1 || action == 3 {
				geom, err := xproto.GetGeometry(X, xproto.Drawable(win)).Reply()
				if err != nil {
					fmt.Println(err)
					break
				}
				if action == 1 { // Move window
					if uint16(v.RootX) + uint16(geom.Width) > screen.WidthInPixels {
						values[0] = uint32(screen.WidthInPixels) - uint32(geom.Width)
					} else {
						values[0] = uint32(v.RootX)
					}
					if uint16(v.RootY) + uint16(geom.Height) > screen.HeightInPixels {
						values[1] = uint32(screen.HeightInPixels) - uint32(geom.Height)
					} else {
						values[1] = uint32(v.RootY)
					}
					xproto.ConfigureWindow(X, win, xproto.ConfigWindowX |
						xproto.ConfigWindowY, values)
				} else { // Resize window
					values[0] = uint32(v.RootX - geom.X)
					values[1] = uint32(v.RootY - geom.Y)
					xproto.ConfigureWindow(X, win, xproto.ConfigWindowWidth |						xproto.ConfigWindowHeight, values)
				}
				X.Sync()
			}
		case xproto.ButtonReleaseEvent:
			fmt.Println("ButtonReleased")
			xproto.UngrabPointer(X, xproto.TimeCurrentTime)
			X.Sync()
		default:
			fmt.Printf("got event %T\n", v)
			fmt.Println("got event", ev)
		}
	}
}
