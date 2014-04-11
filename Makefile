
all:
	go build -o server.bin server.go

run:
	./server.bin

clean:
	rm *.bin
