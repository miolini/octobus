SOURCE=octobus.go
TARGET=octobus

all: clean depends build

clean:
	rm -f $(TARGET)

depends:
	go get -d

build:
	go build -o $(TARGET) $(SOURCE)