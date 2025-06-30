go build -ldflags "-H=windowsgui" -o fivem-windows-amd64.exe .

go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -ico assets/icon.ico -manifest manifest.xml
