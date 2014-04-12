
all:
	go build -o server.bin server.go

run:
	coffee -c .
	ls -l ./public/js/controllers/
	./server.bin

clean:
	rm *.bin
