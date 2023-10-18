
```bash
# @scripthaus command build
GO_LDFLAGS="-s -w -X main.BuildTime=$(date +'%Y%m%d%H%M')"
go build -ldflags="$GO_LDFLAGS" -o bin/mshell-v0.3-darwin.amd64 main-waveshell.go
```

```bash
# @scripthaus command fullbuild
GO_LDFLAGS="-s -w -X main.BuildTime=$(date +'%Y%m%d%H%M')"
go build -ldflags="$GO_LDFLAGS" -o ~/.mshell/mshell-v0.2 main-waveshell.go
GOOS=linux GOARCH=amd64 go build -ldflags="$GO_LDFLAGS" -o bin/mshell-v0.3-linux.amd64 main-waveshell.go
GOOS=linux GOARCH=arm64 go build -ldflags="$GO_LDFLAGS" -o bin/mshell-v0.3-linux.arm64 main-waveshell.go
GOOS=darwin GOARCH=amd64 go build -ldflags="$GO_LDFLAGS" -o bin/mshell-v0.3-darwin.amd64 main-waveshell.go
GOOS=darwin GOARCH=arm64 go build -ldflags="$GO_LDFLAGS" -o bin/mshell-v0.3-darwin.arm64 main-waveshell.go
```

