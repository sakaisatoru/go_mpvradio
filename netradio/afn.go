package netradio

import (
	"context"
	"encoding/xml"
	"fmt"
	"github.com/carlmjohnson/requests"
	"os"
)

const (
	afnurlfile string = "/tmp/afnurl"
)

type Ports struct {
	XMLName xml.Name `xml:"ports"`
	Port    []string `xml:"port"`
}

type Status struct {
	XMLName        xml.Name `xml:"status"`
	Status_code    int      `xml:"status-code"`
	Status_message string   `xml:"status-message"`
}

type Metadata struct {
	XMLName      xml.Name `xml:"metadata"`
	Shoutcast_v1 string   `xml:"shoutcast-v1"`
	Shoutcast_v2 string   `xml:"shoutcast-v2"`
	Sse_sideband string   `xml:"sse-sideband"`
}

type Audio struct {
	XMLName xml.Name `xml:"audio"`
	Codec   string   `xml:"codec,attr"`
}

type Media_format struct {
	XMLName xml.Name `xml:"media-format"`
	Audio   Audio    `xml:"audio"`
}

type Transports struct {
	XMLName   xml.Name `xml:"transports"`
	Transport []string `xml:"transport"`
}

type Server struct {
	XMLName xml.Name `xml:"server"`
	Ip      string   `xml:"ip"`
	Ports   Ports    `xml:"ports"`
}

type Servers struct {
	XMLName xml.Name `xml:"servers"`
	Server  []Server `xml:"server"`
}

type Mountpoint struct {
	XMLName        xml.Name     `xml:"mountpoint"`
	Status         Status       `xml:"status"`
	Tr             Transports   `xml:"transports"`
	Me             Metadata     `xml:"metadata"`
	Servers        Servers      `xml:"servers"`
	Mount          string       `xml:"mount"`
	Format         string       `xml:"format"`
	Bitrate        int          `xml:"bitrate"`
	MediaFormat    Media_format `xml:"media-format"`
	Authentication int          `xml:"authentication"`
	Timeout        int          `xml:"timeout"`
	Send_page_url  int          `xml:"send-page-url"`
}

type Mountpoints struct {
	XMLName xml.Name     `xml:"mountpoints"`
	Mp      []Mountpoint `xml:"mountpoint"`
}

type Live_stream_config struct {
	XMLName xml.Name `xml:"live_stream_config"`
	Xmlns   string   `xml:"xmlns,attr"`
	Version string   `xml:"version,attr"`
}

type afnfeed struct {
	Lsc Live_stream_config `xml:"live_stream_config"`
	Mps Mountpoints        `xml:"mountpoints"`
}

func AFNGetUrlWithApi(station string) (string, error) {
	var u, s string

	url := fmt.Sprintf("https://playerservices.streamtheworld.com/api/livestream?station=%s&transports=http,hls&version=1.8", station)
	err := requests.
		URL(url).
		ToString(&s).
		Fetch(context.Background())
	if err != nil {
		return u, err
	}

	lsc := afnfeed{}
	err = xml.Unmarshal([]byte(s), &lsc)
	if err != nil {
		return u, err
	}

	for _, mountpoint := range lsc.Mps.Mp {
		if mountpoint.MediaFormat.Audio.Codec == "mp3" {
			t, _ := os.ReadFile(afnurlfile)
			cacheurl := string(t)
			newurl := mountpoint.Servers.Server[0].Ip
			for _, v := range mountpoint.Servers.Server {
				if v.Ip == cacheurl {
					newurl = cacheurl
					break
				}
			}
			u = fmt.Sprintf("https://%s/%s.mp3", newurl, station)
			os.WriteFile(afnurlfile, []byte(newurl), 0666)
			break
		}
	}
	return u, err
}
