
package main

import (
	"os"
	"fmt"
	"time"
	"net/http"
	"path/filepath"
	"encoding/json"
	"code.google.com/p/go.net/websocket"
)

// Global variables
var (
	clientCnt uint
	newClient chan *Client
	dbConns chan *Query
	clients map[uint]*Client
	clientTokens map[string]*Client
	rooms map[uint]*Room
)

// ============================================================================
//  Models
// ============================================================================
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
	Q_NEW_CLIENT = iota
	Q_ONLINE
	Q_OFFLINE
	Q_ROOMS 
	Q_MEMBERS
	Q_HISTORY
	Q_JOIN
	Q_LEAVE
	Q_MESSAGE
)
type Query struct {
	action uint
	params interface{}
	receiver chan interface{}
}


// ============================================================================
//  Data access
// ============================================================================
func db() {
	for query := range dbConns{
		var data interface{}
		println("Inside query:", query, query.action, query.params, query.receiver)
		
		switch int((*query).action) {
		case Q_NEW_CLIENT:
			clientCnt++
			client := new(Client)
			client.id = clientCnt
			client.name = fmt.Sprintf("name-%d", clientCnt)
			client.token = fmt.Sprintf("token-%d", clientCnt)
			clients[client.id] = client
			clientTokens[client.token] = client
			data = client
		case Q_ONLINE:
			token := query.params.(string)
			data = clientTokens[token]
		case Q_OFFLINE:
			// ::Later
		case Q_ROOMS:
			idx := 0
			sl := make([](map[string]interface{}), len(rooms))
			for _, room := range rooms {
				sl[idx] = map[string]interface{}{
					"oid": room.id,
					"name": room.name,
				}
				idx ++
			}
			data = sl
			// pass
		case Q_MEMBERS:
			rid := uint(query.params.(float64))
			members := rooms[rid].members
			idx := 0
			sl := make([](map[string]interface{}), len(members))
			for cid, client := range members {
				sl[idx] = map[string]interface{} {
					"oid": cid,
					"name": client.name,
				}
			}
			// pass
		case Q_HISTORY:
			// rid := query.params.(uint)
			// pass
		case Q_JOIN:
			params := query.params.(map[string]interface{})
			rid := uint(params["oid"].(float64))
			client := params["client"].(*Client)
			members := rooms[rid].members
			members[rid] = client
			// pass
		case Q_LEAVE:
			// pass
		default:
			println("Unexcepted query:", query.action)
		}
		
		if query.receiver != nil {
			query.receiver <-data
		}
		println("End query!", query.action)
	}
	
	println("Database closed !!!!!!")
}



// ============================================================================
//  Actions
// ============================================================================

// help function for [createClient, online]
func _newClient() (*Client) {
	receiver := make(chan interface{})
	dbConns <- &Query{Q_NEW_CLIENT, nil, receiver}
	println("waiting for receiver, _newClient")
	data := <-receiver
	client := data.(*Client)
	return client
}

func createClient(req, resp *map[string]interface{}) {
	client := _newClient()
	(*resp)["token"] = client.token
}

func online(req, resp *map[string]interface{}) (*Client) {
	token := (*req)["token"]
	receiver := make(chan interface{})
	dbConns <- &Query{Q_ONLINE, token, receiver}
	data := <-receiver
	println("online.data:", data)

	client := data.(*Client)
	if client == nil {
		println("data is nil.")
		client = _newClient()
	} else {
		println("data is not nil.")
	}
	println("the client:", client)

	(*resp)["oid"] = (*client).id
	(*resp)["name"] = (*client).name
	return client
}

func offline(req, resp *map[string]interface{}) {
	client := (*req)["client"]
	params := map[string]interface{} {
		"client": client,
	}
	dbConns <- &Query{Q_OFFLINE, params, nil}
}

func getRooms(req, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	dbConns <- &Query{Q_ROOMS, nil, receiver}
	rooms := <-receiver
	fmt.Printf("rooms: %v", rooms)
	(*resp)["rooms"] = rooms
}

func members(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	receiver := make(chan interface{})
	dbConns <- &Query{Q_MEMBERS, rid, receiver}
	members := <-receiver
	(*resp)["oid"] = rid
	(*resp)["members"] = members
}

func history(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	receiver := make(chan interface{})
	dbConns <- &Query{Q_HISTORY, rid, receiver}
	messages := <-receiver
	(*resp)["oid"] = rid
	(*resp)["messages"] = messages
}

func join(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	client := (*req)["client"]
	params := map[string]interface{} {
		"oid": rid,
		"client": client,
	}
	dbConns <- &Query{Q_JOIN, params, nil}
	(*resp)["oid"] = rid
}

func leave(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	client := (*req)["client"]
	params := map[string]interface{} {
		"client": client,
		"oid": rid,
	}
	dbConns <- &Query{Q_LEAVE, params, nil}
	(*resp)["oid"] = rid
}

func message(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	client := (*req)["client"]
	params := map[string]interface{} {
		"client" : client,
		"oid" : rid,	// room.id (send message to)
		"body" : (*req)["body"],
		"created_at": time.Now(),
	}
	dbConns <- &Query{Q_MESSAGE, params, nil}
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
			println("[JSON]resp:", string(b))
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
	dbConns = make(chan *Query, CHAN_SIZE)
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
