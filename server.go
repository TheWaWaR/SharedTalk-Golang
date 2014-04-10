
package main

import (
	"os"
	"fmt"
	"time"
	"net/http"
	"encoding/json"
	"path/filepath"
	"code.google.com/p/go.net/websocket"
)

var (
	clientCnt uint
	newClient chan *Client
	dbConn chan *Query
	clients map[uint]*Client
	clientTokens map[string]*Client
	rooms map[uint]*Room
)

// Models
type Client struct {
	id uint
	name string
	token string
}

type Room struct {
	id uint
	name string
	members map[uint]*Client
	history []*Message
}

type Message struct {
	from uint		// client.id
	to uint			// room.id
	body string
	created_at time.Time
}

// Query types
const (				
	Q_ONLINE = iota
	Q_OFFLINE
	Q_ROOMS 
	Q_MEMBERS
	Q_HISTORY
	Q_JOIN
	Q_LEAVE
)
type Query struct {
	action uint
	params interface{}
	receiver chan interface{}
}


// Data access
func db() {
	for true {
		select {
		case client := <-newClient:
			id := client.id
			token := client.token
			clients[id] = client
			clientTokens[token] = client
			
		case query := <-dbConn:
			var data interface{}
			println("Inside query:", query)
			switch int((*query).action) {
			case Q_ONLINE:
				println("Q_LINE", 1)
				token := query.params.(string)
				println("Q_LINE", 2)
				data = clientTokens[token]
				println("Q_LINE", 3, data.(*Client))
			case Q_OFFLINE:
				// pass
			case Q_ROOMS:
				// pass
			case Q_MEMBERS:
				// pass
			case Q_HISTORY:
				// pass
			case Q_JOIN:
				// pass
			case Q_LEAVE:
				// pass
			default:
				println("Unexcepted query:", query.action)
			}
			query.receiver <-data
		}
	}
}


// Actions
func createClient(req, resp *map[string]interface{}) {
	clientCnt += 1
	token := fmt.Sprintf("token-%d", clientCnt)
	client := &Client{clientCnt, fmt.Sprintf("name-%d", clientCnt), token}
	newClient <- client
	(*resp)["token"] = token
}

func online(req, resp *map[string]interface{}) (*Client) {
	token := (*req)["token"].(string)
	params := token
	receiver := make(chan interface{})
	dbConn <- &Query{Q_ONLINE, params, receiver}
	data := <-receiver
	println("online.data:", data)
	client := data.(*Client)
	println("online.client:", client)
	
	(*resp)["oid"] = (*client).id
	(*resp)["name"] = (*client).name
	return client
}

func offline(req, resp *map[string]interface{}) {
	client := (*req)["client"].(*Client)
	params := map[string]interface{} {
		"client": client,
	}
	dbConn <- &Query{Q_OFFLINE, params, make(chan interface{})}
}

func getRooms(req, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	query := &Query{Q_ROOMS, nil, receiver}
	dbConn <-query
	rooms := <-receiver
	(*resp)["rooms"] = rooms
}

func members(req, resp *map[string]interface{}) {
	rid := (*req)["oid"].(int)
	params := map[string]interface{} {
		"id": rid,
	}
	receiver := make(chan interface{})
	query := &Query{Q_MEMBERS, params, receiver}
	dbConn <-query
	members := <-receiver
	(*resp)["oid"] = rid
	(*resp)["members"] = members
}

func history(req, resp *map[string]interface{}) {
	rid := (*req)["oid"].(int)
	params := map[string]interface{} {
		"id": rid,
	}
	receiver := make(chan interface{})
	query := &Query{Q_HISTORY, params, receiver}
	dbConn <-query
	messages := <-receiver
	(*resp)["oid"] = rid
	(*resp)["messages"] = messages
}

func join(req, resp *map[string]interface{}) {
	rid := (*req)["oid"].(int)
	client := (*req)["client"].(*Client)
	params := map[string]interface{} {
		"id": rid,
		"client": client,
	}
	receiver := make(chan interface{})
	query := &Query{Q_JOIN, params, receiver}
	dbConn <-query
	(*resp)["oid"] = rid
}

func leave(req, resp *map[string]interface{}) {
	rid := (*req)["oid"].(int)
	client := (*req)["client"].(*Client)
	params := map[string]interface{} {
		"client": client,
		"id": rid,
	}
	receiver := make(chan interface{})
	query := &Query{Q_LEAVE, params, receiver}
	dbConn <-query
	(*resp)["oid"] = rid
}

func message(req, resp *map[string]interface{}) {
	rid := (*req)["oid"].(int)
	client := (*req)["client"].(*Client)
	params := map[string]interface{} {
		"client" : client,
		"id" : rid,	// room.id (send message to)
		"body" : (*req)["body"],
		"created_at": time.Now(),
	}
	receiver := make(chan interface{})
	query := &Query{Q_LEAVE, params, receiver}
	dbConn <-query
	(*resp)["oid"] = rid
	(*resp)["status"] = "ok"
}

func typing(req, resp *map[string]interface{}) {
	// ::Later
}


// Handler for each client connection
func ChatHandler(ws *websocket.Conn) {
	var client *Client
	buf := make([]byte, 4<<10)
	for true {
		if n, _ := ws.Read(buf); n > 0 {
			println("Received:", n, ":", string(buf[:n]))
			var req map[string]interface{}
			err := json.Unmarshal(buf[:n], &req)
			fmt.Printf("json: %v, %v\n", err, req)
			
			resp := make(map[string]interface{})
			req["client"] = client
			path := req["path"].(string)
			
			// Router
			switch path {
			case "create_client":
				createClient(&req, &resp)
			case "online":
				client = online(&req, &resp)
			case "offline":
				offline(&req, &resp)
				break // Close connection
			case "rooms":
				getRooms(&req, &resp) // already has `rooms`
			case "members":
				members(&req, &resp)
			case "history":
				history(&req, &resp)
			case "join":
				join(&req, &resp)
			case "leave":
				leave(&req, &resp)
			case "message":
				message(&req, &resp)
			case "typing":
				typing(&req, &resp)
			}
			
			resp["path"] = path
			println("response:", resp)
			b, _ := json.Marshal(resp)
			ws.Write(b)
		} else { break }
	}
}


func main() {
	// Serve for static files
	go func() {
		public_root := filepath.Join(filepath.Dir(os.Args[0]), "public")
		http.ListenAndServe(":9090", http.FileServer(http.Dir(public_root)))
	}()

	// Init vars
	CHAN_SIZE := 8
	newClient = make(chan *Client, CHAN_SIZE)
	dbConn = make(chan *Query, CHAN_SIZE)
	clients = make(map[uint]*Client)
	clientTokens = make(map[string]*Client)
	
	rooms = make(map[uint]*Room)
	roomNames := [...]string{"Japan", "China", "U.S.", "Russia", "U.K."}
	for id, name := range roomNames {
		room := &Room{uint(id), name, make(map[uint]*Client), make([]*Message, 50)}
		rooms[uint(id)] = room
	}

	go db()
	
	http.Handle("/", websocket.Handler(ChatHandler))
	http.ListenAndServe(":9091", nil)
}
