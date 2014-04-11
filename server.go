
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

// Constants & Global variables
const (
	DB_POOL_SIZE = 8 
	TIME_LAYOUT = "2006-01-02 15:04:05" // time formatter (weird thing....)
)
var (
	clientCnt uint
	TYPE_MAP map[int]string
	dbPool chan *Query	// Channel for data access
)

// ============================================================================
//  Models
// ============================================================================
type Client struct {
	id uint
	utype int
	name string
	token string
	mailbox chan map[string]interface{}
	rooms map[uint]*Room
}

type User struct {}		// ::TODO
type Visitor struct {}		// ::TODO

type Room struct {
	id uint
	name string
	members map[uint]*Client
	historySize uint
	history []*Message
}

// Sender/Receiver types (for future)
const (
	T_ROOM = iota
	T_USER			// Register user
	T_VISITOR
)
type Message struct {
	from_type, to_type int	// aka. Client.utype
	from_id,   to_id   uint	// client.id ==> room.id
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
	action int
	params interface{}
	receiver chan interface{}
}

// ============================================================================
//  Data access
// ============================================================================
func db() {
	println("[Starting] db() ......")

	clients := make(map[uint]*Client)
	clientTokens := make(map[string]*Client)
	rooms := make(map[uint]*Room)
	roomNames := [...]string{"Japan", "China", "U.S.", "Russia", "U.K."} // tmp variable
	for id, name := range roomNames {
		room := &Room{
			id		: uint(id),
			name		: name,
			members		: make(map[uint]*Client),
			historySize	: 0,
			history		: make([]*Message, 50),
		}
		rooms[uint(id)] = room
	}
	
	for query := range dbPool {
		var data interface{}
		fmt.Printf("[Inside query]: %v\n", query)
		
		switch (*query).action {
		case Q_NEW_CLIENT:
			clientCnt++
			client := new(Client)
			client.id = clientCnt
			client.utype = T_USER
			client.name = fmt.Sprintf("name-%d", clientCnt)
			client.token = fmt.Sprintf("token-%d", clientCnt)
			client.rooms = make(map[uint]*Room)
			clientTokens[client.token] = client
			data = client
			
		case Q_ONLINE:
			token := query.params.(string)
			fmt.Printf("[Q_ONLINE] token: %s\n", token)
			fmt.Printf("[Q_ONLINE] clientTokens: %v\n", clientTokens)
			client, ok := clientTokens[token]
			if ok {
				clients[client.id] = client
				println("[Q_ONLINE] got it")
			}
			data = client
			
		case Q_OFFLINE:
			client := query.params.(*Client)
			for _, room := range client.rooms {
				delete(room.members, client.id)
			}
			delete(clients, client.id)
			
			msg := map[string]interface{} {
				"path": "presence",
				"action": "offline",
				"member": map[string]interface{} {
					"oid": client.id,
				},
			}
			client.mailbox <- msg
			
		case Q_ROOMS:
			idx := 0
			sl := make([](map[string]interface{}), len(rooms))
			for _, room := range rooms {
				sl[idx] = map[string]interface{} {
					"oid": room.id,
					"name": room.name,
				}
				idx ++
			}
			data = sl
			
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
				idx++
			}
			data = sl
			fmt.Printf("[Q_MEMBERS] members: %v\n", rooms[rid].members)
			
		case Q_HISTORY:
			rid := uint(query.params.(float64))
			room := rooms[rid]
			history := room.history
			messages := make([](map[string]interface{}), room.historySize)
			for i := uint(0); i < room.historySize; i++ {
				messages[i] = map[string]interface{} {
					"from_type"	: TYPE_MAP[history[i].from_type],
					"from_id"	: history[i].from_id,
					"to_type"	: TYPE_MAP[history[i].to_type],
					"to_id"		: history[i].to_id,
					"body"		: history[i].body,
					"created_at"	: history[i].created_at.Format(TIME_LAYOUT),
				}
			}
			data = messages
			
		case Q_JOIN:
			params := query.params.(map[string]interface{})
			rid := uint(params["oid"].(float64))
			client := params["client"].(*Client)
			room := rooms[rid]
			client.rooms[rid] = room
			room.members[client.id] = client
			fmt.Printf("[Q_JOIN] rooms[rid].members: %v\n", rooms[rid].members)
			
			msg := map[string]interface{} {
				"path": "presence",
				"action": "join",
				"member": map[string]interface{} {
					"oid": client.id,
					"name": client.name,
				},
			}
			client.mailbox <- msg
			
		case Q_LEAVE:
			params := query.params.(map[string]interface{})
			rid := uint(params["oid"].(float64))
			client := params["client"].(*Client)
			delete(client.rooms, rid)
			delete(rooms[rid].members, client.id)
			
			msg := map[string]interface{} {
				"path" : "presence",
				"action" : "leave",
				"member" : map[string]interface{} {
					"oid" : client.id,
				},
			}
			client.mailbox <- msg
			
		case Q_MESSAGE:
			params := query.params.(map[string]interface{})
			rid := uint(params["to_id"].(float64))			
			client := params["client"].(*Client)
			body := params["body"].(string)
			created_at := params["created_at"].(time.Time)
			message := &Message{
				from_type	: client.utype,
				from_id		: client.id,
				to_type		: T_ROOM,
				to_id		: rid,
				body		: body,
				created_at	: created_at,
			}
			room := rooms[rid]
			room.history[room.historySize] = message
			room.historySize++

			go func () {
				members := room.members
				for _, member := range members {
					msg := map[string]interface{} {
						"path"		: "message",
						"from_type"	: TYPE_MAP[client.utype],
						"from_id"	: client.id,
						"to_type"	: TYPE_MAP[T_ROOM],
						"to_id"		: rid,
						"body"		: body,
						"created_at"	: created_at.Format(TIME_LAYOUT),
					}
					member.mailbox <- msg
				}
			} ()

		default:
			println(">>> Unexcepted query:", query.action)
		}
		
		if query.receiver != nil {
			query.receiver <-data
			close(query.receiver)
		} else {
			println(">>> Receiver is nil:", query.action)
		}
		
		println("[End query]!", query.action)
	}
	println(">>> Database closed !!!!!! Why???")
}

/* ============================================================================
 *  Actions
 * ==========================================================================*/
func createClient(req, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	dbPool <- &Query{Q_NEW_CLIENT, nil, receiver}
	data := <-receiver
	client := data.(*Client)
	(*resp)["token"] = client.token
}

func online(req, resp *map[string]interface{}) (*Client) {
	token := (*req)["token"]
	receiver := make(chan interface{})
	dbPool <- &Query{Q_ONLINE, token, receiver}
	data := <-receiver

	client := data.(*Client)
	println("[Client]:", client)
	if client == nil {
		(*resp)["reset"] = true
	}  else {
		(*resp)["oid"] = (*client).id
		(*resp)["name"] = (*client).name
	}
	
	return client
}

func offline(req, resp *map[string]interface{}) {
	dbPool <- &Query{Q_OFFLINE, (*req)["client"], nil}
}

func getRooms(req, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	dbPool <- &Query{Q_ROOMS, nil, receiver}
	rooms := <-receiver
	(*resp)["rooms"] = rooms
}

func members(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	receiver := make(chan interface{})
	dbPool <- &Query{Q_MEMBERS, rid, receiver}
	members := <-receiver
	(*resp)["oid"] = rid
	(*resp)["members"] = members
}

func history(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	receiver := make(chan interface{})
	dbPool <- &Query{Q_HISTORY, rid, receiver}
	messages := <-receiver
	(*resp)["oid"] = rid
	(*resp)["messages"] = messages
}

func join(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	params := map[string]interface{} {
		"client": (*req)["client"],
		"oid": rid,
	}
	dbPool <- &Query{Q_JOIN, params, nil}
	(*resp)["oid"] = rid
}

func leave(req, resp *map[string]interface{}) {
	rid := (*req)["oid"]
	params := map[string]interface{} {
		"client": (*req)["client"],
		"oid": rid,
	}
	dbPool <- &Query{Q_LEAVE, params, nil}
	(*resp)["oid"] = rid
}

func message(req, resp *map[string]interface{}) {
	to_type := (*req)["type"]
	to_id := (*req)["oid"]
	params := map[string]interface{} {
		"client"	: (*req)["client"],
		"to_type"	: &to_type,
		"to_id"		: to_id, // room.id (send message to)
		"body"		: (*req)["body"],
		"created_at"	: time.Now(),
	}
	dbPool <- &Query{Q_MESSAGE, params, nil}
	(*resp)["status"] = "ok"
}

func typing(req, resp *map[string]interface{}) { /* ::TODO */ }


// Handler for each client connection
func ChatHandler(ws *websocket.Conn) {
	fmt.Printf("[Starting] connection (%v)......\n", ws)
	
	var client *Client
	
	// Receive mssage from internal process (`func db()` actually)
	mailbox := make(chan map[string]interface{}, 2)
	go func() {
		for msg := range mailbox {
			fmt.Printf("[Mailbox]: %v\n", msg)
			path := msg["path"].(string)
			switch path {
			case "message":
				// pass
			case "presence":
				// pass
			default:
				println("[Mailbox] Something wrong here!!! ==>", path)
			}
			b, _ := json.Marshal(msg)
			ws.Write(b)
		}
	} ()
	
	buf := make([]byte, 32<<10)
	for true {
		if n, _ := ws.Read(buf); n > 0 {
			// Decode request
			println("[Received]:", n, ":", string(buf[:n]))
			var req map[string]interface{}
			json.Unmarshal(buf[:n], &req)
			
			resp := make(map[string]interface{})
			req["client"] = client
			path := req["path"].(string)
			
			// Request Router
			switch path {
			case "create_client":
				createClient(&req, &resp)
			case "online":
				client = online(&req, &resp)
				if client != nil {
					client.mailbox = mailbox
				}
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

			// Encode response
			resp["path"] = path
			b, _ := json.Marshal(resp)
			ws.Write(b)
			println("[Response]:", string(b))
			
		} else { break }
	}
}


func main() {

	// Init vars
	TYPE_MAP = map[int]string {
		T_ROOM : "room",
		T_USER : "user",
		T_VISITOR : "visitor",
	}
	dbPool = make(chan *Query, DB_POOL_SIZE)
	go db()
	
	// Serve for static files
	go func() {
		println("[Starting] static files server......")
		public_root := filepath.Join(filepath.Dir(os.Args[0]), "public")
		http.ListenAndServe(":9090", http.FileServer(http.Dir(public_root)))
	}()
	
	http.Handle("/", websocket.Handler(ChatHandler))
	http.ListenAndServe(":9091", nil)
}
