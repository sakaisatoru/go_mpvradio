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
    "path"
    "path/filepath"
    "slices"
    "local.packages/netradio"
    "local.packages/mpvctl"
    "time"
    "flag"
)

const (
	PACKAGE			string = "go_mpvradio"
	PACKAGE_VERSION	string = "0.1.1"
	appID 			string = "com.google.endeavor2wako.go_mpvradio"
	stationlist 	string = "/usr/local/share/mpvradio/playlists/radio.m3u"
	MPV_SOCKET_PATH string = "/run/user/1000/mpvsocket"
	ICON_DIR_PATH	string = "mpvradio/logo"
	APP_ICON		string = "pixmaps/mpvradio.png"
	PLAYLISTS		string = "mpvradio/playlists/*.m3u"
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
	mpvheaderbar *gtk.HeaderBar
	inputarea *gtk.Box
	mpvret = make(chan string)
	mu sync.Mutex
	last_selected_station string = ""
	tabletmode bool
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
				mpvheaderbar.SetSubtitle(ms.Data)
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

	s := fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", "/usr/local/share/mpvradio/sounds/button57.mp3")
	err = mpvctl.Send(s)
	time.Sleep(300*time.Millisecond)
	s = fmt.Sprintf("{\"command\": [\"loadfile\",\"%s\"]}\x0a", station_url)
	err = mpvctl.Send(s)
	radio_enable = true	
}

/*
 * playlist の検索を行う
 */
func getplaylists() ([]string, error) {
	var(
		files []string
		err error
		dirs []string
	)
	dirs = append(dirs, xdg.ConfigHome)
	dirs = append(dirs, xdg.DataDirs...)
	
	for _, v := range dirs {
		d := filepath.Join(v, PLAYLISTS)
		//~ fmt.Printf("check dir : %s\n", d)
		files, err = filepath.Glob(d)
		if err != nil {
			fmt.Println(err)
			continue
		}
		if len(files) >= 1 {
			break
		}
	}
	return files, err
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
			//~ fmt.Println("make stateless action")
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
									// 同一局の連続接続を抑止する。複数のスタックに跨って判断可能なように
									// 大域変数を使う
									if st != last_selected_station {
										mpvheaderbar.SetSubtitle(st) //
										u, _ := panel.store[st]
										last_selected_station = st
										tune(u)
									}
								}
							})
					}
				})
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

	for k := range keys {
		label, err := gtk.LabelNew(keys[k])
		if err == nil {
			box,_ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 2)
			icon_f := fmt.Sprintf("%s.png", filepath.Join(xdg.CacheHome, ICON_DIR_PATH, keys[k]))
			_, width, height, err := gdk.PixbufGetFileInfo(icon_f)
			if err != nil {
				t0, _ := panel.store[keys[k]]
				icon_f = fmt.Sprintf("%s.png", filepath.Join(xdg.CacheHome, ICON_DIR_PATH, path.Base(t0)))
				_, width, height, _ = gdk.PixbufGetFileInfo(icon_f)
			}
			if width > 144 {width = 144}
			if height > 64 {height = 64}
			
			var icon *gtk.Image
			pbuf, err := gdk.PixbufNewFromFileAtSize(icon_f, width, height)
			if err == nil {
				icon, err = gtk.ImageNewFromPixbuf(pbuf)
				if err != nil {
					icon, _ = gtk.ImageNewFromIconName(PACKAGE, gtk.ICON_SIZE_DIALOG)
				}
			} else {
				icon, _ = gtk.ImageNewFromIconName(PACKAGE, gtk.ICON_SIZE_DIALOG)
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
		volbtn.SetValue(0.7)
		// ストップボタン
		stopbtn, err := gtk.ButtonNewFromIconName("media-playback-stop-symbolic",
													gtk.ICON_SIZE_BUTTON)
		if err != nil {
			return win, err
		}
		stopbtn.Connect("clicked", func() {	mpvctl.Stop()
											radio_enable = false
											last_selected_station = ""
											mpvheaderbar.SetSubtitle("") })
		
		mpvheaderbar,err = gtk.HeaderBarNew()
		if err != nil {
			return win,err
		}
		mpvheaderbar.SetDecorationLayout("menu:close")
		mpvheaderbar.SetShowCloseButton(true)
		mpvheaderbar.SetTitle(PACKAGE)
		mpvheaderbar.SetHasSubtitle(true)
		win.SetTitlebar(mpvheaderbar)
		mpvheaderbar.PackEnd (volbtn)
		mpvheaderbar.PackEnd (stopbtn)
		
		// stack
		notebook,err := gtk.StackNew()
		if err != nil {
			return win,err
		}
		//~ notebook.SetTransitionType(gtk.STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT)
		files,err := getplaylists()
		if err == nil && len(files) >= 1 {
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
				notebook.AddTitled(scroll, l.GetLabel(), l.GetLabel())
			}
		}
		
		// input area
		inputarea,err = gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 3)
		if err != nil {
			return win,err
		}
		entrybuffer,err := gtk.EntryBufferNew("",-1)
		if err != nil {
			return win,err
		}
		btn_tune,_ := gtk.ButtonNewWithLabel("Tune Now")
		btn_tune.Connect("clicked",func() { 
			if url, err := entrybuffer.GetText(); err == nil {
				tune(url)
				inputarea.Hide()
			}})
		btn_cancel,_ := gtk.ButtonNewWithLabel("Cancel")
		btn_cancel.Connect("clicked", (*gtk.Widget)(unsafe.Pointer(inputarea)).Hide)
		entry,_ := gtk.EntryNewWithBuffer(entrybuffer)
		entry.SetPlaceholderText("ここにURLを入力してください")
		inputarea.PackStart(entry,true,true,0)
		inputarea.PackEnd(btn_cancel,false,false,0)
		inputarea.PackEnd(btn_tune,false,false,0)

		box,err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL,2)
		if err != nil {
			return win, err
		}

		if tabletmode {
			// タブレット向けレイアウト
			box2,err := gtk.BoxNew(gtk.ORIENTATION_VERTICAL,2)
			if err != nil {
				return win, err
			}
			sw,err := gtk.StackSwitcherNew()
			if err != nil {
				return win,err
			}
			box3,err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL,1)
			if err != nil {
				return win,err
			}
			notebook.SetTransitionType(gtk.STACK_TRANSITION_TYPE_SLIDE_LEFT_RIGHT)
			sw.SetStack(notebook)
			box3.PackStart(sw, true,false,1)
			box2.PackStart(box3, false, true, 5)
			box2.PackStart(notebook, true, true, 0)
			box.PackStart(inputarea, false, true, 0)
			box.PackStart(box2, true, true, 0)
		} else {
			// デスクトップ向けレイアウト
			box2,err := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL,2)
			if err != nil {
				return win, err
			}
			sw,err := gtk.StackSidebarNew()
			if err != nil {
				return win,err
			}
			notebook.SetTransitionType(gtk.STACK_TRANSITION_TYPE_SLIDE_UP_DOWN)
			sw.SetStack(notebook)
			box2.PackStart(sw, false, true, 0)
			box2.PackStart(notebook, true, true, 0)
			box.PackStart(inputarea, false, true, 0)
			box.PackStart(box2, true, true, 0)
		}
		win.Add(box)
		win.SetDefaultSize(800, 600)
		win.Connect("show",func() {inputarea.Hide()})
	}
	return win, err
}

func main() {
	app, err := gtk.ApplicationNew(appID, glib.APPLICATION_HANDLES_COMMAND_LINE)
	if err != nil {
		log.Fatal(err)
	}

	if err := mpvctl.Init(MPV_SOCKET_PATH);err != nil {
		log.Fatal(err)
	}

	app.Connect("command-line", func()  {
		flag.BoolVar(&tabletmode, "tablet", false, "タブレットモードで起動する")
		flag.Parse()
		app.Activate()
	})
	
	app.Connect("startup", func() {
		if err := mpvctl.Open(MPV_SOCKET_PATH);err != nil {
			fmt.Println("time out.", err)	// time out
			app.Quit()
		}
		radio_enable = false
		volume = 60
		go mpvctl.Recv(cb_mpvrecv)
		
		builder,err := gtk.BuilderNewFromString (`<interface>
		<!-- interface-requires gtk+ 3.0 -->
		<menu id="appmenu">
		<section>
		  <item>
			<attribute name="label" translatable="yes">QuickTune</attribute>
			<attribute name="action">app.quicktune</attribute>
		  </item>
		  <item>
			<attribute name="label" translatable="yes">_About</attribute>
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
				{"quicktune",func(action *glib.SimpleAction) {
						if inputarea.GetVisible() == true {
							inputarea.Hide()
						} else {inputarea.Show()}
				},"","",nil},
				{"about", about_activated, "", "", nil},
				{"quit", func(action *glib.SimpleAction) {app.Quit()}, "", "", nil},
			}
		action_map_add_action_entries (app, app_entries)
		app.SetAccelsForAction("app.quit", []string{"<Ctrl>Q"})
		app.SetAccelsForAction("app.quicktune", []string{"<Ctrl>L"})
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
			if tabletmode {
				w.Maximize()
			}
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
