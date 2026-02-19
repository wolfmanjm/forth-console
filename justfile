alias b := builds

# build for all targets
[group('build')]
builds:
    GOOS=linux GOARCH=amd64 go build -o bins/forthcon-linux
    GOOS=windows GOARCH=amd64 go build -o bins/forthcon.exe
    GOOS=darwin GOARCH=arm64 go build -o bins/forthcon-macarm
    GOOS=darwin GOARCH=amd64 go build -o bins/forthcon-macamd
    GOOS=linux GOARCH=arm64 go build -o bins/forthcon-rpi

[group('build')]
buildudp:
    GOOS=linux GOARCH=amd64 go build -C cmd  -o udp-send-linux
    GOOS=windows GOARCH=amd64 go build -C cmd  -o udp-send.exe
    GOOS=darwin GOARCH=arm64 go build -C cmd  -o udp-send-macarm
    GOOS=darwin GOARCH=amd64 go build -C cmd  -o udp-send-macamd
    GOOS=linux GOARCH=arm64 go build -C cmd  -o udp-send-rpi

# do a static check on the entire project
check:
    staticcheck ./...
