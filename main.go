package main

import (
	"fmt"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xinerama"
	"github.com/BurntSushi/xgb/xproto"
)

// ICCCM related atoms
var (
	atomWMProtocols    xproto.Atom
	atomWMDeleteWindow xproto.Atom
	atomWMTakeFocus    xproto.Atom
)

var activeWindow *xproto.Window
var windows []xproto.Window

func addWin (X *xgb.Conn, w xproto.Window) error {
	// Ensure that we can manage this window.
	if err := xproto.ConfigureWindowChecked(
		X,
		w,
		xproto.ConfigWindowBorderWidth,
		[]uint32{
			0,
		}).Check(); err != nil {
		return err
	}

	// Get notifications when this window is deleted.
	if err := xproto.ChangeWindowAttributesChecked(
		X,
		w,
		xproto.CwEventMask,
		[]uint32{
			xproto.EventMaskStructureNotify |
			xproto.EventMaskEnterWindow,
		}).Check(); err != nil {
		return err
	}

	windows = append(windows, w)

	return nil
}

func rmWin (w xproto.Window) {
	index := -1
    for i, v := range windows {
        if v == w {
            index = i
            break
        }
    }
    if index != -1 {
        windows[index] = windows[len(windows)-1]
        windows = windows[:len(windows)-1]
    }
}

func organize (X *xgb.Conn) {
	for _, w := range windows {
		err := xproto.ConfigureWindowChecked(X, w, xproto.ConfigWindowX |
			xproto.ConfigWindowY | xproto.ConfigWindowWidth |
			xproto.ConfigWindowHeight, []uint32{0, 0, 512, 512}).Check()
		if err != nil {
			fmt.Println(err)
		}
	}
}

func getAtom (X *xgb.Conn, name string) xproto.Atom {
	rply, err := xproto.InternAtom(X, false, uint16(len(name)), name).Reply()
	if err != nil {
		fmt.Println(err)
	}
	if rply == nil {
		return 0
	}
	return rply.Atom
}

func main () {
	values := []uint32{0, 0, 0}
	X, err := xgb.NewConnDisplay(":0")
	if err != nil {
		panic(err)
	}

	var win xproto.Window
	info := xproto.Setup(X)

	if err := xinerama.Init(X); err != nil {
			panic(err)
	}

	screen := info.DefaultScreen(X)
	root := screen.Root

	atomWMProtocols    = getAtom(X, "WM_PROTOCOLS")
	atomWMDeleteWindow = getAtom(X, "WM_DELETE_WINDOW")
	atomWMTakeFocus    = getAtom(X, "WM_TAKE_FOCUS")

	err = xproto.ChangeWindowAttributesChecked(
		X,
		root,
		xproto.CwEventMask,
		[]uint32{
			xproto.EventMaskKeyPress |
			xproto.EventMaskKeyRelease |
			xproto.EventMaskButtonPress |
			xproto.EventMaskButtonRelease |
			xproto.EventMaskStructureNotify |
			xproto.EventMaskSubstructureRedirect,
		}).Check()
	if err != nil {
		if _, ok := err.(xproto.AccessError); ok {
			fmt.Println("Could not become the WM. Is another WM already running?")
			panic(err)
		}
	}

	xproto.GrabKey(X, true, root, xproto.ModMask1, 67,
		xproto.GrabModeAsync, xproto.GrabModeAsync)

	xproto.GrabButton(X, false, root,
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync, xproto.GrabModeAsync, root, 0, 1, xproto.ModMask1)

	xproto.GrabButton(X, false, root,
		xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease,
		xproto.GrabModeAsync, xproto.GrabModeAsync, root, 0, 3, xproto.ModMask1)

	tree, err := xproto.QueryTree(X, root).Reply()
	if err != nil {
		panic(err)
	}
	if tree != nil {
		for _, c := range tree.Children {
			if err := addWin(X, c); err != nil {
				fmt.Println(err)
			}

		}
		organize(X)
	}

	//X.Sync()

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
		case xproto.DestroyNotifyEvent:
			fmt.Println("Destroyed window!")
			rmWin(v.Window)
			organize(X)
			// TODO find last window and activate it
			_, err := xproto.SetInputFocusChecked(X,
				xproto.InputFocusPointerRoot, root,
				xproto.TimeCurrentTime).Reply()
			if err != nil {
				fmt.Println(err)
			}
		case xproto.ConfigureRequestEvent:
			fmt.Println("Configure Window!")
			ev := xproto.ConfigureNotifyEvent{
				Event:            v.Window,
				Window:           v.Window,
				AboveSibling:     0,
				X:                v.X,
				Y:                v.Y,
				Width:            v.Width,
				Height:           v.Height,
				BorderWidth:      0,
				OverrideRedirect: false,
			}
			xproto.SendEventChecked(X, false, v.Window,
				xproto.EventMaskStructureNotify, string(ev.Bytes()))
		case xproto.MapRequestEvent:
			fmt.Println("MapRequestEvent")
			if winattrib, err := xproto.GetWindowAttributes(X, v.Window).Reply(); err != nil || !winattrib.OverrideRedirect {
				xproto.MapWindowChecked(X, v.Window)
				addWin(X, v.Window)
				organize(X)
			}
		case xproto.EnterNotifyEvent:
			fmt.Println("EnterNotify!")
			activeWindow = &v.Event

			prop, err := xproto.GetProperty(X, false, v.Event, atomWMProtocols,
				xproto.GetPropertyTypeAny, 0, 64).Reply()
			focused := false
			if err == nil {
			TakeFocusPropLoop:
				for x := prop.Value; len(x) >= 4; x = x[4:] {
					switch xproto.Atom(uint32(x[0]) | uint32(x[1])<<8 | uint32(x[2])<<16 | uint32(x[3])<<24) {
					case atomWMTakeFocus:
						xproto.SendEventChecked(
							X,
							false,
							v.Event,
							xproto.EventMaskNoEvent,
							string(xproto.ClientMessageEvent{
								Format: 32,
								Window: *activeWindow,
								Type:   atomWMProtocols,
								Data: xproto.ClientMessageDataUnionData32New([]uint32{
									uint32(atomWMTakeFocus),
									uint32(v.Time),
									0,
									0,
									0,
								}),
							}.Bytes())).Check()
						focused = true
						break TakeFocusPropLoop
					}
				}
			}
			if !focused {
				if _, err := xproto.SetInputFocusChecked(X,
					xproto.InputFocusPointerRoot, v.Event,
					v.Time).Reply(); err != nil {
					fmt.Println(err)
				}
			}
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
			//X.Sync()
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
				//X.Sync()
			}
		case xproto.ButtonReleaseEvent:
			fmt.Println("ButtonReleased")
			xproto.UngrabPointer(X, xproto.TimeCurrentTime)
			//X.Sync()
		case xproto.KeyReleaseEvent:
			fmt.Println("Key released!")
			if v.Detail == 65 {
				fmt.Println("Quiting!")
				return
			}
		default:
			fmt.Printf("got event %T\n", v)
			fmt.Println("got event", ev)
		}
	}
}
