package mpvctl

import (
		"fmt"
		"net"
		"time"
		"os/exec"
		"strings"
		"encoding/json"
)

//~ type MpvIRCdata struct {
	//~ Filename	*string		`json:"filename"`
	//~ Current		bool		`json:"current"`
	//~ Playing		bool		`json:"playing"`
//~ }
 
type MpvIRC struct {
    //~ Data       	*MpvIRCdata	 `json:"data"`
    Data       	string	 `json:"data"`
    Name		string	 `json:"name"`
	Request_id  int	 	 `json:"request_id"`
    Err 		string	 `json:"error"`
    Event		string	 `json:"event"`
}

func (ms *MpvIRC) clear() {
	ms.Data = "" 
    ms.Name = ""
	ms.Request_id = 0
    ms.Err = ""		
    ms.Event = ""		
}

const (
	IRCbuffsize int = 1024
	MPVOPTION1     string = "--idle"
	MPVOPTION2     string = "--input-ipc-server="
	MPVOPTION3     string = "--no-video"
	MPVOPTION4     string = "--no-cache"
	MPVOPTION5     string = "--stream-buffer-size=256KiB"
	//~ MPVOPTION6	   string = "--script=/home/pi/bin/title_trigger.lua"
	MPVOPTION6	   string = ""
)

var (
	mpv net.Conn
	mpvprocess *exec.Cmd
	//~ volconv = []int8{	 0, 2, 4, 5, 6,  7, 8, 9,10,11,
						//~ 12,13,13,14,15,	16,16,17,18,18,
						//~ 19,20,21,22,23,	24,25,26,27,28,
						//~ 29,30,32,33,35,	36,38,40,42,45,
						//~ 47,50,53,57,61,	66,71,78,85,99}

	volconv = []int8{	0,15,24,30,35, 39,42,45,48,50,
						52,54,56,57,59, 60,62,63,64,65,
						66,67,68,69,70, 71,72,72,73,74,
						75,75,76,77,77, 78,78,79,80,80,
						81,81,82,82,83, 83,84,84,85,85,
						85,86,86,87,87, 87,88,88,89,89,
						89,90,90,90,91, 91,91,92,92,92,
						93,93,93,93,94, 94,94,95,95,95,
						95,96,96,96,96, 97,97,97,97,98,
						98,98,98,99,99, 99,99,100,100,100}

	Volume_steps = len(volconv)
	Volume_min int8 = 0
	Volume_max int8 = int8(Volume_steps - 1)
	readbuf = make([]byte, IRCbuffsize)
	Cb_connect_stop = func() bool { return false }
)

func Init(socketpath string) error {
	mpvprocess = exec.Command("/usr/bin/mpv", 	MPVOPTION1, 
												MPVOPTION2+socketpath, 
												MPVOPTION3, MPVOPTION4, 
												//~ MPVOPTION5, MPVOPTION6)
												MPVOPTION5)
	err := mpvprocess.Start()
	return err
}

func Mpvkill() error {
	err := mpvprocess.Process.Kill()
	return err
}

func Open(socket_path string) error {
	var err error
	for i := 0; ;i++ {
		mpv, err = net.Dial("unix", socket_path)
		if err == nil {
			break
		}
		time.Sleep(200*time.Millisecond)
		if i > 60 {
			return err	// time out
		}
	}
	return nil
}

func Close() {
	mpv.Close()
}

func Send(s string) error {
	_, err := mpv.Write([]byte(s))
	return err
}

//~ func Recv(ch chan<- string, cb func(MpvIRC) (string, bool)) {
func Recv(cb func(MpvIRC) (string, bool)) {
	var ms MpvIRC
	
	for {
		n, err := mpv.Read(readbuf)
		if err == nil {
			if n < IRCbuffsize {
				for _, s := range(strings.Split(string(readbuf[:n]),"\n")) {
					if len(s) > 0 {
						ms.clear() // 中身を消さないとフィールド単位で持ち越される場合がある
						err := json.Unmarshal([]byte(s),&ms)
						if err == nil {
							cb(ms)
							//~ s, ok := cb(ms)
							//~ if ok  {
								//~ ch <- s
							//~ }
						}
					}
				}
			}
		}
	}
}

func Setvol(vol int8) {
	if vol < Volume_min {
		vol = Volume_min
	} else if vol > Volume_max {
		vol = Volume_max
	} 
	//~ fmt.Printf("vol=%d\n",vol)
	s := fmt.Sprintf("{\"command\": [\"set_property\",\"volume\",%d]}\x0a",volconv[vol])
	Send(s)
}

func Stop() {
	if Cb_connect_stop() == false {
		Send("{\"command\": [\"stop\"]}\x0a")
	}
}

