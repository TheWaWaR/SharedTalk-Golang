
all:
	go build -o server.bin server.go

deps:
	go get code.google.com/p/go.net/websocket
	npm install -g coffee-script

run:
	coffee -c .
	ls -l ./public/js/controllers/
	./server.bin

clean:
	rm *.bin
