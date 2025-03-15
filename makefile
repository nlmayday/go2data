# 编译Linux 64位
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/linux-amd64 .

# 编译Windows 64位
build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/windows-amd64.exe .

# 清理
clean:
	rm -rf bin

.PHONY: build-linux build-windows clean