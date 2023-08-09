export GOOS=linux
export GOARCH=amd64
go build -o kubetop main.go
#CGO_ENABLED=0 go build -a -tags netgo -ldflags '-w -extldflags "-static"' -o kubetop main.go
