module github.com/fwilkerson/practices

go 1.27.2

require github.com/google/go-cmp v0.7.0

tool (
	github.com/fwilkerson/practices/cmd/guide
	github.com/fwilkerson/practices/cmd/health
	github.com/fwilkerson/practices/cmd/size
	github.com/fwilkerson/practices/cmd/style
)
