#! /usr/bin/env python3

#afn.py
import socket, os, time
from sys import argv

def get_url_with_api():
	import requests, re
	ip = re.compile("<ip>(.+?)</ip>")

	# URLを指定
	url = "https://playerservices.streamtheworld.com/api/livestream?station={0}&transports=http,hls&version=1.8".format(argv[1])

	# リクエスト送信
	response = requests.get(url)

	m = re.search(ip,response.text)
	if m != None:
		u = '{0}/{1}'.format(m.group(1),argv[1])
	else:
		u = argv[1]
	return u

MPVSOCKET='/run/user/1000/mpvsocket'
RECVSIZE=1024
afn_url_cachedir = os.path.join(os.path.expanduser("~"), ".cache","mpvradio","afn","")
read_api = False

os.makedirs(afn_url_cachedir, exist_ok=True)

afn_url_cache = os.path.join(afn_url_cachedir, argv[1])
if os.path.isfile(afn_url_cache):
	print("ローカルキャッシュを読んでいます")
	with open(afn_url_cache) as f:
		u = f.read()
else:
	print("web apiを呼んでいます")
	u = get_url_with_api()
	read_api = True
	
s2="{\"command\": [\"loadfile\", \"http://%s.mp3\"]}\n" % u

# ~ print("s2", s2)

s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.connect(MPVSOCKET)
s.send(s2.encode())
d = s.recv(RECVSIZE)
# ~ print(d)

time.sleep(5)
s2 = '{\"command\": [\"get_property\",\"playlist\"]}\n'
s.send(s2.encode())
d = s.recv(RECVSIZE)
# ~ print("mpv status", str(d), len(d))

s.close()


if '"file_error":"loading failed"' in str(d):
	# error
	if not read_api:
		os.remove(afn_url_cache)
		print("無効なurlキャッシュを削除しました。")
	print("plugin afn.py : 接続に失敗しました。しばらくしてから再度試してください。")
else:
	with open(afn_url_cache, mode='w') as f:
		# ~ print("save ",u)
		f.write(u)
		

