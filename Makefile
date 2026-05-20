.PHONY: build clean install all

BINARY := pi-stream

all: build

build:
	go build -o $(BINARY) .

clean:
	rm -f $(BINARY)

install: build
	cp $(BINARY) ~/.local/bin/
