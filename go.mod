module go_mpvradio/go_mpvradio

go 1.22.1

replace local.packages/netradio => ./netradio

require (
	github.com/gotk3/gotk3 v0.6.3
	local.packages/netradio v0.0.0-00010101000000-000000000000
)

require (
	github.com/carlmjohnson/requests v0.23.5 // indirect
	golang.org/x/net v0.15.0 // indirect
)
