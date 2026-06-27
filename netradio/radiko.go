package netradio

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"github.com/carlmjohnson/requests"
	"net/http"
	//~ "net/url"
	//~ "os"
	"crypto/md5"
	//~ "regexp"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	auth_key  string = "bcd151073c03b352e1ef2fd66c32209da9ca0afa" // 現状は固有 key_lenght = 0
	tokenfile string = "/tmp/radiko_token"
	userAgent string = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"

	auth1Url string = "https://radiko.jp/v2/api/auth1"
	auth2Url string = "https://radiko.jp/v2/api/auth2"

	RADIKOPORXYPORT string = ":18080"
	RADIKOPORXYADDR string = "/live.m3u8"
)

type Url struct {
	XMLName           xml.Name `xml:"url"`
	Areafree          int      `xml:"areafree,attr"`
	Timefree          int      `xml:"timefree,attr"`
	PlaylistCreateUrl string   `xml:"playlist_create_url"`
}

type Urls struct {
	XMLName xml.Name `xml:"urls"`
	List    []Url    `xml:"url"`
}

type RadikoURL struct {
	M3u8Url   string
	Token     string
	UserAgent string
}

type RadikoProxy struct {
	Addr   string
	Svr    RadikoURL
	stop   bool
	client *http.Client
}

func RadikoProxyNew() *RadikoProxy {
	return &RadikoProxy{
		Addr:   "http://localhost" + RADIKOPORXYPORT + RADIKOPORXYADDR,
		Svr:    RadikoURL{},
		stop:   true,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (sv *RadikoProxy) SetStationInfo(v *RadikoURL) {
	sv.Svr = *v
}

func (sv *RadikoProxy) GetProxyAddress() string {
	return sv.Addr
}

func (sv *RadikoProxy) IsStop() bool {
	return sv.stop
}

func (sv *RadikoProxy) Start() {
	sv.stop = false

	// http://localhost:18080/live.m3u8 へのリクエストを処理
	http.HandleFunc(RADIKOPORXYADDR, func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequest("GET", sv.Svr.M3u8Url, nil)
		req.Header.Set("X-Radiko-AuthToken", sv.Svr.Token)
		req.Header.Set("User-Agent", sv.Svr.UserAgent)
		resp, err := sv.client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})

	go func() {
		if err := http.ListenAndServe(RADIKOPORXYPORT, nil); err != nil {
			fmt.Printf("Proxy server failed: %v", err)
		}
	}()
}

func RadikoGetUrl(station string) (*RadikoURL, error) {
	var (
		s, regionInfo, m3u8List string
		m3u8Urls                Urls
	)

	radikoURL := &RadikoURL{
		M3u8Url:   "",
		Token:     "",
		UserAgent: userAgent,
	}

	h := http.Header{}
	h.Add("User-Agent", radikoURL.UserAgent)
	h.Add("Accept", "*/*")
	h.Add("X-Radiko-App", "pc_html5")
	h.Add("X-Radiko-App-Version", "0.0.1")
	h.Add("X-Radiko-User", "dummy_user")
	h.Add("X-Radiko-Device", "pc")

	h2 := http.Header{}
	err := requests.
		URL(auth1Url).
		Headers(h).
		CopyHeaders(h2).
		ToString(&s).
		Fetch(context.Background())
	if err != nil {
		return radikoURL, err
	}

	radikoURL.Token = h2.Get("x-radiko-authtoken")
	offset, _ := strconv.Atoi(h2.Get("x-radiko-keyoffset"))
	length, _ := strconv.Atoi(h2.Get("x-radiko-keylength"))
	partialkey := base64.StdEncoding.EncodeToString([]byte(auth_key[offset : offset+length]))

	h3 := http.Header{}
	h3.Add("X-Radiko-AuthToken", radikoURL.Token)
	h3.Add("X-Radiko-Partialkey", partialkey)
	h3.Add("X-Radiko-User", "dummy_user")
	h3.Add("X-Radiko-Device", "pc")

	h4 := http.Header{}
	err = requests.
		URL(auth2Url).
		Headers(h3).
		CopyHeaders(h4).
		ToString(&regionInfo).
		Fetch(context.Background())
	if err != nil {
		return radikoURL, err
	}
	//~ fmt.Println("リージョン", regionInfo)

	h5 := http.Header{}
	h5.Add("X-Radiko-AuthToken", radikoURL.Token)
	h5.Add("User-Agent", radikoURL.UserAgent)
	h6 := http.Header{}
	err = requests.
		URL(fmt.Sprintf("https://radiko.jp/v3/station/stream/pc_html5/%s.xml", station)).
		Headers(h5).
		CopyHeaders(h6).
		ToString(&m3u8List).
		Fetch(context.Background())
	if err != nil {
		return radikoURL, err
	}
	//~ fmt.Println("urlリスト", m3u8List)

	err = xml.Unmarshal([]byte(m3u8List), &m3u8Urls)
	if err != nil {
		return radikoURL, err
	}
	for _, u0 := range m3u8Urls.List {
		if u0.Areafree == 0 && u0.Timefree == 0 {
			hash := md5.Sum([]byte("mpvradio_" + time.Now().Format(time.DateTime)))
			nu := fmt.Sprintf("%s?station_id=%s&l=15&lsid=%x&type=b",
				u0.PlaylistCreateUrl, station, hash)

			h10 := http.Header{}
			h10.Add("X-Radiko-AuthToken", radikoURL.Token)
			h10.Add("User-Agent", radikoURL.UserAgent)
			h11 := http.Header{}
			sss := ""
			err = requests.
				URL(nu).
				Headers(h10).
				CopyHeaders(h11).
				ToString(&sss).
				Fetch(context.Background())
			pos := strings.Index(sss, "https://")
			radikoURL.M3u8Url = strings.TrimRight(sss[pos:], "\n")
			//~ fmt.Println(radikoURL.M3u8Url)
			break
		}
	}
	return radikoURL, err
}
