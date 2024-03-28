### 编译
#### Mac
`CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o jms_inspect pkg/cmd/inspect.go`

#### Windows
`CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o jms_inspect.exe pkg/cmd/inspect.go`

#### Linux
`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o jms_inspect pkg/cmd/inspect.go`