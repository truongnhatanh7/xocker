build:
	go build -o bin/xocker main.go

run: build
	sudo ./bin/xocker run /bin/sh "echo Hello Xocker"
