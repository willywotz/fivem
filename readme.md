go build -ldflags "-s -w -H=windowsgui" -o fivem-windows-amd64.exe .

go run github.com/akavel/rsrc@latest -ico icon.ico -manifest manifest.xml
go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@latest -64 -file-version "v0" -product-version "v0"
