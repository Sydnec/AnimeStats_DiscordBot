module github.com/Sydnec/AnimeStats_DiscordBot

go 1.25.0

// Installée telle quelle par setup-go (go-version-file), minimum imposé ailleurs
// par GOTOOLCHAIN=auto : sans elle, la CI et les releases compilaient avec
// Go 1.25.0, privé de tous ses correctifs de sécurité. La directive go reste le
// minimum du langage ; celle-ci est à relever quand govulncheck signale la
// bibliothèque standard.
toolchain go1.27.3

require (
	github.com/bwmarrin/discordgo v0.29.0
	github.com/robfig/cron/v3 v3.0.1
	golang.org/x/time v0.14.0
	modernc.org/sqlite v1.58.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b // indirect
	golang.org/x/sys v0.47.0 // indirect
	modernc.org/libc v1.75.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)
