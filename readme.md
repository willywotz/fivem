go build -ldflags "-H=windowsgui" .

go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -ico assets/icon.ico -manifest manifest.xml
