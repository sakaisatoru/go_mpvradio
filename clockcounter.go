package main

import (
	"fmt"
	"github.com/gotk3/gotk3/gtk"
)

type digitWheel struct {
	*gtk.Box
	number       *gtk.Label
	up           *gtk.Button
	down         *gtk.Button
	n            int8
	max0         int8
	min0         int8
	changedValue func(a int8)
}

func digitWheelNew() *digitWheel {
	v, _ := gtk.LabelNew("")
	u, _ := gtk.ButtonNewFromIconName("go-up-symbolic", 24)
	d, _ := gtk.ButtonNewFromIconName("go-down-symbolic", 24)
	b, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	w := &digitWheel{
		Box:    b,
		number: v,
		up:     u,
		down:   d,
		n:      0,
		max0:   9,
		min0:   0,
	}
	w.changedValue = func(a int8) {}
	w.number.Set("use-markup", true)
	w.up.Connect("clicked", func(b *gtk.Button) {
		w.n++
		if w.n > w.max0 {
			w.n = w.min0
		}
		w.SetValue(int(w.n))
	})
	w.down.Connect("clicked", func(b *gtk.Button) {
		w.n--
		if w.n < w.min0 {
			w.n = w.max0
		}
		w.SetValue(int(w.n))
	})
	w.PackStart(u, false, false, 0)
	w.PackStart(v, false, false, 0)
	w.PackStart(d, false, false, 0)

	return w
}

func (w *digitWheel) GetValue() int {
	return int(w.n)
}

func (w *digitWheel) SetValue(a int) {
	w.n = int8(a)
	w.number.SetLabel(fmt.Sprintf("<span size='large' weight='bold'>% 2d</span>", w.n))
	w.changedValue(w.n)
}

type digitClock struct {
	*gtk.Box
	hh *digitWheel
	hl *digitWheel
	mh *digitWheel
	ml *digitWheel
}

func digitClockNew() *digitClock {
	b, _ := gtk.BoxNew(gtk.ORIENTATION_HORIZONTAL, 0)
	dc := &digitClock{
		Box: b,
		hh:  digitWheelNew(),
		hl:  digitWheelNew(),
		mh:  digitWheelNew(),
		ml:  digitWheelNew(),
	}

	dc.hh.max0 = 2
	dc.mh.max0 = 5

	dc.hh.changedValue = func(a int8) {
		switch a {
		case 2:
			dc.hl.max0 = 3
			n0 := dc.hl.GetValue()
			if n0 > int(dc.hl.max0) {
				n0 = 3
			}
			dc.hl.SetValue(int(n0))
		case 1, 0:
			dc.hl.max0 = 9
		}
	}

	colon, _ := gtk.BoxNew(gtk.ORIENTATION_VERTICAL, 0)
	colonLabel, _ := gtk.LabelNew("")
	colonLabel.SetMarkup("<span size='large' weight='bold'>:</span>")
	colon.PackStart(colonLabel, true, true, 0)

	dc.PackStart(dc.hh, false, false, 0)
	dc.PackStart(dc.hl, false, false, 0)
	dc.PackStart(colon, false, false, 0)
	dc.PackStart(dc.mh, false, false, 0)
	dc.PackStart(dc.ml, false, false, 0)

	return dc
}

// GetValue 設定時刻を返す。戻り値のフォーマットはtime.TimeOnlyで秒は常に00が返る。
func (dc *digitClock) GetValue() string {
	return fmt.Sprintf("%01d%01d:%01d%01d:00",
		dc.hh.GetValue(),
		dc.hl.GetValue(),
		dc.mh.GetValue(),
		dc.ml.GetValue())
}

// SetValue 時刻を設定する。t はtime.TimeOnlyで、秒は無視される。
func (dc *digitClock) SetValue(t string) {
	dc.hh.SetValue(int(t[0] - '0'))
	dc.hl.SetValue(int(t[1] - '0'))
	dc.mh.SetValue(int(t[3] - '0'))
	dc.ml.SetValue(int(t[4] - '0'))
}
