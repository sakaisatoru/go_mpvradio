package main

import (
    "github.com/gotk3/gotk3/gtk"
    "github.com/gotk3/gotk3/gdk"
    "github.com/gotk3/gotk3/glib"
    "github.com/adrg/xdg"
    "log"
    "strings"
    "bufio"
    "fmt"
    "unsafe"
    "os"
    "path/filepath"
    //~ "time"
    "slices"
    "local.packages/netradio"
    "local.packages/mpvctl"
)

const (
	PACKAGE			string = "go_mpvradio"
	appID 			string = "com.google.endeavor2wako.go_mpvradio"
	stationlist 	string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/user/1000/mpvsocket"
	ICON_DIR_PATH	string = "mpvradio/logo"
)

var (
	child_selected_change bool = false
	stlist map[string]string = make(map[string]string)	
	radio_enable bool
	volume int8
	mpvmessagebuffer *gtk.EntryBuffer
	mpvret = make(chan string)
)

// mpvからの応答を選別するフィルタ
func cb_mpvrecv(ms mpvctl.MpvIRC) (string, bool) {
	//~ fmt.Printf("%#v\n",ms)
	if radio_enable {
		if ms.Event == "property-change" {
			if ms.Name == "metadata/by-key/icy-title" {
				//~ fmt.Println(ms.Data)
				mpvmessagebuffer.SetText(ms.Data)
				return ms.Data, true
			}
		}
	}
	return "", false
}

func tune(url string) {
	var (
		station_url string
		err error = nil
	)
	
	args := strings.Split(url, "/")
	if args[0] == "plugin:" {
		switch args[1] {
			case "afn.py":
				station_url, err = netradio.AFN_get_url_with_api(args[2])
			case "radiko.py":
				station_url, err = netradio.Radiko_get_url(args[2])
			default:
				break
		}
		if err != nil {
			return 
		}
	} else {
		station_url = url
	}

	s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", station_url)
	mpvctl.Send(s)
	radio_enable = true	
}

func setup_station_list () {
	file, err := os.Open(stationlist)
	if err != nil {
		log.Fatal(err)
	} 
	defer file.Close()

	scanner := bufio.NewScanner(file)
	f := false
	s := ""
	name := ""
	for scanner.Scan() {
		s = scanner.Text()
		if strings.Contains(s, "#EXTINF:") == true {
			f = true
			_, name, _ = strings.Cut(s, "/")
			name = strings.Trim(name, " ")
			continue
		}
		if f {
			if len(s) != 0 {
				f = false
				stlist[name] = s
			//~ fmt.Printf("station name : %s  data : %s\n", name,s)
			}
		}
	}
}

func cb_isbox(wi *gtk.Widget) {
	a, err := wi.GetName()
	if err == nil {
		if a == "GtkBox" {
			container_foreach((*gtk.Container)(unsafe.Pointer(wi)), func(w2 *gtk.Widget) {
				p, err := w2.GetName()
				if err == nil {
					if p == "GtkLabel" {
						u, _ := stlist[(*gtk.Label)(unsafe.Pointer(w2)).GetLabel()]
						tune(u)
					}
				}
			})
		}
	}
}

func container_foreach(container *gtk.Container, cb func(wi *gtk.Widget) ) {
	//~ fmt.Printf("container_foreach\n")
	list := container.GetChildren()
	current := list
	for current != nil {
		p, ok := current.Data().(*gtk.Widget)
		if ok {
			cb(p)
		}
		current = current.Next()
	}
	list.Free()
}

func child_activate_cb (box *gtk.FlowBox, child *gtk.FlowBoxChild) {
	if child_selected_change {
		container_foreach((*gtk.Container)(unsafe.Pointer(child)), cb_isbox)
	}
}

func radiopanel_new () (*gtk.FlowBox, error) {
    grid, err := gtk.FlowBoxNew()
    if err == nil {
		grid.SetSelectionMode(gtk.SELECTION_SINGLE)
		grid.SetHomogeneous(true)
		grid.SetActivateOnSingleClick(true)
		grid.SetColumnSpacing(2)
		grid.SetMaxChildrenPerLine(6)
		// ラベルにてボタンクリックと等価の動作を行うための準備
		grid.Connect ("child-activated", child_activate_cb)
		// カーソルキーで移動する毎に生じるイベント
		grid.Connect ("selected-children-changed", func() {
									child_selected_change = true})
		// playlist_table をチェックして選局ボタンを並べる
		var keys []string
		for k, _ := range stlist {
			keys = append(keys, k)
		}
		slices.Sort(keys)

		for _,k := range keys {
			label, err := gtk.LabelNew(k)
			if err == nil {
				box,_ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
				icon_f := fmt.Sprintf("%s.png", filepath.Join(xdg.CacheHome, ICON_DIR_PATH, k))
				_, width, height, err := gdk.PixbufGetFileInfo(icon_f)
				if err == nil {
					if width > 200 {width = 200}
					if height > 64 {height = 64}
				}
				var icon *gtk.Image
				var er error
				pbuf, err := gdk.PixbufNewFromFileAtSize(icon_f, width, height)
				if err != nil {
					icon, _ = gtk.ImageNewFromIconName(PACKAGE, gtk.ICON_SIZE_DIALOG)
				} else {
					icon, er = gtk.ImageNewFromPixbuf(pbuf)
					if er != nil {
						icon, _ = gtk.ImageNewFromIconName(PACKAGE, gtk.ICON_SIZE_DIALOG)
					}
				}
				box.PackStart(icon,false,false,2)
				box.PackStart(label,false,false,2)
				grid.Insert(box,-1)
			}
		}
	}
    return grid, nil;
}

func mpvradio_window_new(app *gtk.Application) (*gtk.ApplicationWindow, error) {
	// build gui 
    win, err := gtk.ApplicationWindowNew(app)
    if err == nil {
		// ボリュームボタン
		volbtn, err := gtk.VolumeButtonNew();
		if err != nil {
			return win, err
		}
		volbtn.Connect("value-changed", func(volbtn *gtk.VolumeButton, v float64) {
			mpvctl.Setvol(int8(v*float64(mpvctl.Volume_steps)))
		})
		volbtn.SetValue(0.25)
		// ストップボタン
		stopbtn, err := gtk.ButtonNewFromIconName("media-playback-stop-symbolic",
													gtk.ICON_SIZE_BUTTON);
		if err != nil {
			return win, err
		}
		stopbtn.Connect("clicked", func() {mpvctl.Stop(); radio_enable = false})
		
		header,err := gtk.HeaderBarNew()
		if err == nil {
			header.SetDecorationLayout("menu:close")
			header.SetShowCloseButton(true)
			header.SetTitle(PACKAGE);
			header.SetHasSubtitle(true)
			win.SetTitlebar(header)
			header.PackEnd (volbtn)
			header.PackEnd (stopbtn)
		} else {
			win.SetTitle(PACKAGE)
		}
		
		fbox,err := radiopanel_new()
		if err != nil {
			return win, err
		}
		scroll,err := gtk.ScrolledWindowNew(nil,nil)
		if err != nil {
			return win, err
		}
		scroll.SetKineticScrolling(true);
		scroll.SetCaptureButtonPress(true);
		scroll.SetOverlayScrolling(true);
		scroll.SetPolicy(gtk.POLICY_AUTOMATIC, gtk.POLICY_AUTOMATIC)
		scroll.Add(fbox)

		mpvmessagebuffer,_ = gtk.EntryBufferNew("",-1)
		mpvmessage,err := gtk.EntryNewWithBuffer(mpvmessagebuffer)
		if err != nil {
			return win, err
		}
		mpvmessage.SetCanFocus(false)

		box,err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL,2)
		if err != nil {
			return win, err
		}
		box.PackStart(mpvmessage, false, false, 0)
		box.PackStart(scroll, true, true, 0)

		win.Add(box)
		win.SetDefaultSize(800, 600)
		
	}
	return win, err
}

func main() {
	if err := mpvctl.Init(MPV_SOCKET_PATH);err != nil {
		log.Fatal(err)
	}
	
	app, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application.", err)
	}
	
	app.Connect("startup", func() {
		setup_station_list()
		if err := mpvctl.Open(MPV_SOCKET_PATH);err != nil {
			fmt.Println("time out.", err)	// time out
			app.Quit()
		}
		radio_enable = false
		volume = 60
		fmt.Println("Start up.");
		go mpvctl.Recv(cb_mpvrecv)
	})

	app.Connect("shutdown", func() {
		windows := app.GetWindows()
		for windows != nil {
				fmt.Println("try1")
			d := windows.Data()
			w, ok := d.(*gtk.Window)
			if ok {
				fmt.Println("try2")
				if !w.InDestruction() {
					// window size saving routine here.
					fmt.Println("destory")
					w.Destroy()
				}
			}
			windows = windows.Next()
		}
		mpvctl.Close()
		mpvctl.Mpvkill()
		err := os.Remove(MPV_SOCKET_PATH)
        if err != nil {
			fmt.Println(err)
		}
		fmt.Println("shutdown.")
	})

	app.Connect("activate", func() {
		//~ fmt.Println("activate")
		windows := app.GetWindows()
		if windows == nil {
			w, err := mpvradio_window_new(app)
			if err != nil {
				app.Quit()
			} else {
				w.Connect("destroy",func() {
					fmt.Println("window destroy")
				})
				s := "{ \"command\": [\"observe_property_string\", 1, \"metadata/by-key/icy-title\"] }"
				mpvctl.Send(s)
				w.ShowAll()
				w.Present()
			}
		} else {
			d := windows.Data()
			w, ok := d.(*gtk.Window)
			if ok {
				w.Present()
			}
		}
		fmt.Println("activate.");
	})

	app.Run(os.Args)
}
