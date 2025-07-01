go build -ldflags "-s -w -H=windowsgui" -o fivem-windows-amd64.exe .

go run github.com/akavel/rsrc@latest -arch amd64 -ico scripts/icon.ico -manifest scripts/manifest.xml
