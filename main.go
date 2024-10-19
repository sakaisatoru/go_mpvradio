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
    "sync"
    "path/filepath"
    "slices"
    "local.packages/netradio"
    "local.packages/mpvctl"
)

const (
	PACKAGE			string = "go_mpvradio"
	PACKAGE_VERSION	string = "0.1.0"
	appID 			string = "com.google.endeavor2wako.go_mpvradio"
	stationlist 	string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/user/1000/mpvsocket"
	ICON_DIR_PATH	string = "mpvradio/logo"
	APP_ICON		string = "pixmaps/mpvradio.png"
)

type actionEntry struct {
	name string
	activate func(action *glib.SimpleAction)
	parameter_type string
	state string
	change_state func(action *glib.SimpleAction) 
}

type radioPanel struct {
	grid *gtk.FlowBox
	store map[string]string
	child_selected_change bool
}

var (
	radio_enable bool
	volume int8
	mpvmessagebuffer *gtk.EntryBuffer
	mpvret = make(chan string)
	mu sync.Mutex
)

func about_activated(action *glib.SimpleAction) {
	if dialog, err := gtk.AboutDialogNew();err == nil {
		if logofile,err := xdg.SearchDataFile(APP_ICON);err == nil {
			if buf,err := gdk.PixbufNewFromFile(logofile);err == nil {
				dialog.SetLogo(buf)
			}
		}
		dialog.SetCopyright("endeavor wako 2024")
		dialog.SetAuthors([]string{"endeavor wako","sakai satoru"})
		dialog.SetProgramName(PACKAGE)
		dialog.SetTranslatorCredits("endeavor wako (japanese)")
		dialog.SetLicenseType(gtk.LICENSE_LGPL_2_1)
		dialog.SetVersion(PACKAGE_VERSION)
		result := dialog.Run()
		if result == gtk.RESPONSE_DELETE_EVENT {
			dialog.Destroy()
		}
	}
}

// mpvからの応答を選別するフィルタ
func cb_mpvrecv(ms mpvctl.MpvIRC) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	if radio_enable {
		if ms.Event == "property-change" {
			if ms.Name == "metadata/by-key/icy-title" {
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

/*
 * gotk3 にラッパーがないので go で書いた g_action_map_add_action_entries()
 * https://github.com/GNOME/glib/blob/main/gio/gactionmap.c
 * 
 * goのコールバックは引数を取らないので、parameterは扱えない。
 */
func action_map_add_action_entries (app *gtk.Application, entries []actionEntry) {
	var(
		action *glib.SimpleAction
		parameter_type *glib.VariantType
		state *glib.Variant
	)

	for i := 0; i < len(entries); i++ {
		if entries[i].parameter_type != "" {
			if glib.VariantTypeStringIsValid(entries[i].parameter_type) {
				fmt.Printf(`"critical: g_action_map_add_entries: the type "
                          "string '%s' given as the parameter type for "
                          "action '%s' is not a valid GVariant type "
                          "string.  This action will not be added."`,
                          entries[i].parameter_type, entries[i].name)
			}
			parameter_type = nil
		}
		
		if entries[i].state != "" {
			parameter_type = glib.VARIANT_TYPE_STRING
			state = glib.VariantFromString(entries[i].state)
			action = glib.SimpleActionNewStateful(entries[i].name, parameter_type, state)
		} else {
			fmt.Println("make stateless action")
			parameter_type = nil
			action = glib.SimpleActionNew(entries[i].name, parameter_type)
		}
		
		if entries[i].activate != nil {
			action.Connect("activate", entries[i].activate)
		}
		
		if entries[i].change_state != nil {
			action.Connect("change-state", entries[i].change_state)
		}
		app.AddAction(action)
	}
}

/*
 * gotk3 にラッパーが無いので go で書いた gtk_container_foreach ()
 */
func container_foreach(container *gtk.Container, cb func(wi *gtk.Widget) ) {
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

//~ func radiopanel_new (stl map[string]string) (*gtk.FlowBox, error) {
func radiopanel_new(playlistfile string) (*radioPanel, error) {
    panel := new(radioPanel)
    panel.store = make(map[string]string)
    panel.child_selected_change = false

    err := panel.readPlayList(playlistfile)
    if err != nil {
		return panel, err
	}
    panel.grid, err = gtk.FlowBoxNew()
    if err != nil {
		return panel, err
	}
	panel.grid.SetSelectionMode(gtk.SELECTION_SINGLE)
	panel.grid.SetHomogeneous(true)
	panel.grid.SetActivateOnSingleClick(true)
	panel.grid.SetColumnSpacing(2)
	panel.grid.SetMaxChildrenPerLine(6)
	// ラベルにてボタンクリックと等価の動作を行うための準備
	panel.grid.Connect ("child-activated", 
		func(box *gtk.FlowBox, child *gtk.FlowBoxChild) {
			if panel.child_selected_change {
				panel.child_selected_change = false
				container_foreach((*gtk.Container)(unsafe.Pointer(child)), 
					func(wi *gtk.Widget) {
						a, err := wi.GetName()
						if err == nil && a == "GtkBox" {
							container_foreach((*gtk.Container)(unsafe.Pointer(wi)), 
								func(w2 *gtk.Widget) {
									p, err := w2.GetName()
									if err == nil && p == "GtkLabel" {
										st := (*gtk.Label)(unsafe.Pointer(w2)).GetLabel()
										mpvmessagebuffer.SetText(st)
										u, _ := panel.store[st]
										tune(u)
									}
								})
						}
					})
			}
		})
	
	// カーソルキーで移動する毎に生じるイベント
	panel.grid.Connect ("selected-children-changed", func() {
								panel.child_selected_change = true})
	// playlist_table をチェックして選局ボタンを並べる
	var keys []string
	for k, _ := range panel.store {
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
				if width > 144 {width = 144}
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
			panel.grid.Insert(box,-1)
		}
	}
    return panel, nil;
}

func (panel radioPanel) readPlayList(listfile string) error {
	file, err := os.Open(listfile)
	if err != nil {
		return err
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
				panel.store[name] = s
			//~ fmt.Printf("station name : %s  data : %s\n", name,s)
			}
		}
	}
	return nil
}

func mpvradio_window_new(app *gtk.Application) (*gtk.ApplicationWindow, error) {
	// build gui 
    win, err := gtk.ApplicationWindowNew(app)
    if err == nil {
		// ボリュームボタン
		volbtn, err := gtk.VolumeButtonNew()
		if err != nil {
			return win, err
		}
		volbtn.Connect("value-changed", func(volbtn *gtk.VolumeButton, v float64) {
			mpvctl.Setvol(int8(v*float64(mpvctl.Volume_steps)))
		})
		volbtn.SetValue(0.25)
		// ストップボタン
		stopbtn, err := gtk.ButtonNewFromIconName("media-playback-stop-symbolic",
													gtk.ICON_SIZE_BUTTON)
		if err != nil {
			return win, err
		}
		stopbtn.Connect("clicked", func() {mpvctl.Stop(); radio_enable = false})
		
		header,err := gtk.HeaderBarNew()
		if err == nil {
			header.SetDecorationLayout("menu:close")
			header.SetShowCloseButton(true)
			header.SetTitle(PACKAGE)
			header.SetHasSubtitle(true)
			win.SetTitlebar(header)
			header.PackEnd (volbtn)
			header.PackEnd (stopbtn)
		} else {
			win.SetTitle(PACKAGE)
		}
		
		// notebook 
		notebook,err := gtk.NotebookNew()
		if err != nil {
			return win,err
		}
		
		files, err:=filepath.Glob("/home/sakai/.config/mpvradio/playlists/*.m3u")
		if err != nil {
			fmt.Println(err)
		}
		for _, v := range files {
			fbox,err := radiopanel_new(v)
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
			scroll.Add(fbox.grid)
			l, _ := gtk.LabelNew(filepath.Base(v))
			if notebook.AppendPage(scroll, l) < 0 {
				fmt.Printf("%s append error.\n",v)
			}
		}
		notebook.SetTabPos(gtk.POS_LEFT)
		notebook.SetScrollable(true)

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
		box.PackStart(notebook, true, true, 0)

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
		if err := mpvctl.Open(MPV_SOCKET_PATH);err != nil {
			fmt.Println("time out.", err)	// time out
			app.Quit()
		}
		radio_enable = false
		volume = 60
		go mpvctl.Recv(cb_mpvrecv)
		
		//~ <section>
		  //~ <item>
			//~ <attribute name="label" translatable="yes">QuickTune</attribute>
			//~ <attribute name="action">app.quicktune</attribute>
		  //~ </item>
		  //~ <item>
			//~ <attribute name="label" translatable="yes">StatusIcon</attribute>
			//~ <attribute name="action">app.statusicon</attribute>
		  //~ </item>
		//~ </section>

		builder,err := gtk.BuilderNewFromString (`<interface>
		<!-- interface-requires gtk+ 3.0 -->
		<menu id="appmenu">
		<section>
		  <item>
			<attribute name="label" translatable="yes">_about</attribute>
			<attribute name="action">app.about</attribute>
		  </item>
		  <item>
			<attribute name="label" translatable="yes">_Quit</attribute>
			<attribute name="action">app.quit</attribute>
		  </item>
		</section>
		</menu>
		</interface>`)
		if err != nil {
			fmt.Printf("%s\n", err)
			return
		} 
		app_entries := []actionEntry{
				{"about", about_activated, "", "", nil},
				{"quit", func(action *glib.SimpleAction) {app.Quit()}, "", "", nil},
			}
		action_map_add_action_entries (app, app_entries)
		app.SetAccelsForAction("app.quit", []string{"<Ctrl>Q"})
		app_menu,err := builder.GetObject("appmenu")
		if err != nil {
			fmt.Printf("%s\n", err)
			return
		} 
		if m,ok := app_menu.(*glib.MenuModel);ok {
			app.SetAppMenu(m)
		}
		fmt.Println("Start up.");
	})

	app.Connect("shutdown", func() {
		mpvctl.Close()
		mpvctl.Mpvkill()
		err := os.Remove(MPV_SOCKET_PATH)
        if err != nil {
			fmt.Println(err)
		}

		windows := app.GetWindows()
		if windows != nil {
			windows.Foreach( func(e interface{}) {
				if w,ok := e.(*gtk.Window);ok {
					if w.InDestruction() == false {
						w.Destroy()
					}
				}
			})
		}
		fmt.Println("shutdown.")
	})

	app.Connect("activate", func() {
		windows := app.GetWindows()
		if windows == nil {
			w, err := mpvradio_window_new(app)
			if err != nil {
				app.Quit()
			} 
			w.Connect("destroy",func() {
				fmt.Println("destroy now.")
			})
			s := "{ \"command\": [\"observe_property_string\", 1, \"metadata/by-key/icy-title\"] }"
			mpvctl.Send(s)
			w.ShowAll()
			w.Present()
		} else {
			d := windows.Data()
			if w, ok := d.(*gtk.Window); ok {
				w.Present()
			}
		}
		fmt.Println("activate.");
	})

	app.Run(os.Args)
}
