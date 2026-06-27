package netradio

import (
	"os"
	"bufio"
	"strings"
)

type StationInfo struct {
	Name string
	Url  string
}

func PrepareStationList(st string) ([]*StationInfo, error) {
	var (
		file   *os.File
		err    error
		stlist []*StationInfo
	)
	file, err = os.Open(st)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	f := false
	s := ""
	name := ""
	extflag := false

	for scanner.Scan() {
		s = strings.TrimLeft(scanner.Text(), " ")
		if strings.Contains(s, "#EXTM3U") {
			extflag = true
			continue
		}
		if strings.Contains(s, "#EXTINF:") && extflag {
			f = true
			_, name, _ = strings.Cut(s, "/")
			name = strings.Trim(name, " ")
			continue
		}
		if len(s) != 0 {
			if s[:1] == "#" {
				continue
			}
			stmp := new(StationInfo)
			stmp.Url = s
			if f {
				f = false
				// UTF-8 対応で rune　で数える
				stmp.Name = string([]rune(name + "                ")[:16])
			} else {
				stmp.Name = ""
			}
			stlist = append(stlist, stmp)
		}
	}
	return stlist, err
}

func Radiko_setup(stlist []*StationInfo) {
	for _, st := range stlist {
		args := strings.Split(st.Url, "/")
		if args[0] == "plugin:" {
			if args[1] == "radiko.py" {
				_, _ = Radiko_get_url(args[2])
				break
			}
		}
	}
}
