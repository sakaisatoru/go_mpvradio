module go_mpvradio/go_mpvradio

go 1.25.5

replace local.packages/netradio => ./netradio

replace local.packages/mpvctl => ./mpvctl

replace local.packages/preferences => ./preferences

require (
	github.com/adrg/xdg v0.5.3
	github.com/gotk3/gotk3 v0.6.5-0.20251124190141-e7a9e823ca35
	local.packages/mpvctl v0.0.0-00010101000000-000000000000
	local.packages/netradio v0.0.0-00010101000000-000000000000
	local.packages/preferences v0.0.0-00010101000000-000000000000
)

require (
	github.com/carlmjohnson/requests v0.25.1 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)
