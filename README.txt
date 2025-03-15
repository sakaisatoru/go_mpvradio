i686 向けクロスビルド

sudo apt install libgtk-3-dev:i386
CC=i686-linux-gnu-gcc CXX=i686-linux-gnu-g++ CGO_ENABLED=1 CGO_LDFLAGS="-L/usr/lib/i386-linux-gnu" GOOS=linux GOARCH=386 go build
