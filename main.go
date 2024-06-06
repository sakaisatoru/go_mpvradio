package main

import (
    "github.com/gotk3/gotk3/gtk"
    "github.com/gotk3/gotk3/glib"
    "log"
    "strings"
    "bufio"
    "fmt"
    "unsafe"
    "net"
    "os"
    "os/exec"
    "time"
)

const (
	PACKAGE			string = "go_mpvradio"
	appID 			string = "com.google.endeavor2wako.go_mpvradio"
	stationlist 	string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	PLUGINSDIR		string = "/usr/local/share/mpvradio/plugins/"
	MPV_SOCKET_PATH string = "/run/user/1000/mpvsocket"
	MPVOPTION1     	string = "--idle"
	MPVOPTION2     	string = "--input-ipc-server="+MPV_SOCKET_PATH
	MPVOPTION3     	string = "--no-video"
	MPVOPTION4     	string = "--no-cache"
	MPVOPTION5     	string = "--stream-buffer-size=256KiB"
	MPVOPTION6	   	string = "--script=/home/pi/bin/title_trigger.lua"
	mpvIRCbuffsize 	int = 1024
)

var (
	child_selected_change bool = false
	mpv	net.Conn
	stlist map[string]string = make(map[string]string)	
	radio_enable bool
	readbuf = make([]byte, mpvIRCbuffsize)
	mpvprocess *exec.Cmd
	volume int8
)

func mpv_send(s string) {
	mpv.Write([]byte(s))
	for {
		n, err := mpv.Read(readbuf)
		if err != nil {
			log.Println(err)
		}
		//~ fmt.Println(string(readbuf[:n]))
		if n < mpvIRCbuffsize {
			break
		}
	}
}

func mpv_setvol(vol float64) {
	if vol < 1 {
		vol = 0
	} else if vol >= 100 {
		vol = 99
	} 
	//~ fmt.Println("volume:",vol)
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a", int(vol))
	mpv_send(s)
}

func tune(url string) {
	args := strings.Split(url, "/")
	if args[0] == "plugin:" {
		cmd := exec.Command(PLUGINSDIR+args[1], args[2])
		err := cmd.Run()
		if err != nil {
			radio_enable = false
		} else {
			radio_enable = true
		}
	} else {
		s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", url)
		mpv_send(s)
		radio_enable = true
	}
}

func radio_stop() {
	mpv_send("{\"command\": [\"stop\"]}\x0a")
	radio_enable = false
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
			}
		}
	}
}


func child_activate_cb (box *gtk.FlowBox, child *gtk.FlowBoxChild) {
	if child_selected_change {
		child_selected_change = false
		list := child.GetChildren()
		current := list
		for current != nil {
			data := current.Data()
			if data != nil {
				p, ok := data.(*gtk.Widget)
				if ok {
					a, err := p.GetName()
					if err == nil {
						if a == "GtkLabel" {
							l := (*gtk.Label)(unsafe.Pointer(p))
							n := l.GetLabel()
							u, _ := stlist[n]
							tune(u)
						} else {
							fmt.Println("need GtkLabel, but ", a)
						}
					} else {
						fmt.Println(err)
					}
				}
			}
			current = current.Next()
		}
		list.FreeFull(func(item interface{}) {
			v := item.(*gtk.Widget)
			v.Unref()})
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
		for k, _ := range stlist {
			label, err := gtk.LabelNew(k)
			if err == nil {
				grid.Insert(label,-1)
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
			mpv_setvol(v*100)
		})
		volbtn.SetValue(50)
		// ストップボタン
		stopbtn, err := gtk.ButtonNewFromIconName("media-playback-stop-symbolic",
													gtk.ICON_SIZE_BUTTON);
		if err != nil {
			return win, err
		}
		stopbtn.Connect("clicked", radio_stop)
		
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

		box,err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL,2)
		if err != nil {
			return win, err
		}
		box.PackStart(scroll, true, true, 0)

		win.Add(box)
		win.SetDefaultSize(800, 600)
		
		// オプション
		// 上下カーソルキーに音量調整を割り当てる
		//~ if (g_key_file_get_boolean (kconf, "mode", "allowkey_volume", NULL)) {
			//~ g_signal_connect (G_OBJECT(window), "key-press-event",
						//~ G_CALLBACK(mainwindow_key_press_event_cb), NULL);
		//~ }
	}
	return win, err
}

func main() {
	app, err := gtk.ApplicationNew(appID, glib.APPLICATION_FLAGS_NONE)
	if err != nil {
		log.Fatal("Could not create application.", err)
	}
	
	app.Connect("startup", func() {
		//~ fmt.Println("startup")
		mpvprocess = exec.Command("/usr/bin/mpv", 	MPVOPTION1, MPVOPTION2, 
													MPVOPTION3, MPVOPTION4, 
													MPVOPTION5)
													//~ MPVOPTION5, MPVOPTION6)
		mpvprocess.Start()
		setup_station_list()
		var err error
		for i := 0; ;i++ {
			mpv, err = net.Dial("unix", MPV_SOCKET_PATH);
			if err == nil {
				break
			}
			time.Sleep(200*time.Millisecond)
			if i > 60 {
				fmt.Println("time out.", err)	// time out
				app.Quit()
			}
		}
		radio_enable = false
		volume = 60
	})

	app.Connect("shutdown", func() {
		//~ fmt.Println("shutdown")
		windows := app.GetWindows()
		for windows != nil {
			d := windows.Data()
			w, ok := d.(*gtk.Window)
			if ok {
				if !w.InDestruction() {
					// window size saving routine here.
					w.Destroy()
				}
			}
			windows = windows.Next()
		}
		mpv.Close()
		mpvprocess.Process.Kill()
		err := os.Remove(MPV_SOCKET_PATH)
        if err != nil {
			fmt.Println(err)
		}
	})

	app.Connect("activate", func() {
		//~ fmt.Println("activate")
		windows := app.GetWindows()
		if windows == nil {
			w, err := mpvradio_window_new(app)
			if err != nil {
				app.Quit()
			} else {
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
	})
	app.Run(os.Args)
}
