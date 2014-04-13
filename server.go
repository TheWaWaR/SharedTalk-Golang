
package main

// TODO:
//   1. Time Zone issue
//   2. Data persistent (Then we can use `mother goose')
//   3. Command line arguments
//   4. Register-User/Visitor support


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
	QUERY_BUF_SIZE = 8 
	TIME_LAYOUT = "2006-01-02 15:04:05" // time formatter (weird thing....)
)
var (
	clientCnt	 uint
	TYPE_MAP	 map[uint64]string
	dbQuerys	 chan *Query // Channel for data access
)

/*=============================================================================
 * Models
 *===========================================================================*/
type Client struct {
	id	 uint64
	utype	 uint64
	name	 string
	token	 string
	mailbox	 chan map[string]interface{}
	rooms	 map[uint64]*Room
}

type User struct {}		// ::TODO
type Visitor struct {}		// ::TODO

type Room struct {
	id		 uint64
	name		 string
	members		 map[uint64]*Client
	history		 []*Message
}

// Sender/Receiver types (::TODO)
const (
	// Can be ==> [0x1 ... 0xF]
	T_ROOM = uint64(0)
	T_USER = uint64(0x1<<60) // Register user
	T_VISITOR = uint64(0x2<<60) // Anonymous user from other website, the pattern will be like this: https://www.zopim.com/
)
type Message struct {
	from_type, to_type uint64	// aka. Client.utype
	from_id,   to_id   uint64	// client.id ==> room.id
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
	action		 int
	params		 interface{}
	receiver	 chan interface{}
}

/*============================================================================
 * [DATA ACCESS]:
 *     Receive some arguments then send a map[string]interface{} back
 
  
 * Session: (User1, User2) or (User2, User1)
 *    Session.id = encodeSessionId(User1.type, User1.id, User2.type, User2.id)
 
 * Special query requirements (Pagination required):
 * ------------------------------------------------
 *   1. [Get] 通过获取某房间的所有消息记录.
 *        (Room.id) ==> []*Message
 *   2. [Get] 获取和某用户的所有消息记录.
 *        (UserMe.utype, UserMe.id, UserOther.utype, UserOther.id) ==> []*Message
 *   3. [Search] 通过注册用户的用户名或匿名用户的位置搜索用户
 *        (Ruser.name || Vuser.location) ==> []*User
 *   4. [Search] 通过消息内容搜索所有和我相关的消息
 *        (UserMe.utype, UserMe.id, Message.body) ==> []*Message

 * 哪些是非常频繁的操作?
 * --------------------
 *   1. 更新在线用户列表 ==> 可以伸缩的定长数组
 *   2. 更新房间的成员列表 ==> 可以伸缩的定长数组
 *   3. 新增消息 ==> 不得不放数据库并建好索引
 *   4. 获取房间或Session的历史记录 ==> 不得不放数据库并建好索引
 *==========================================================================*/

var reverseBits = [...]uint64 {
	0x00, 0x80, 0x40, 0xC0, 0x20, 0xA0, 0x60, 0xE0, 0x10, 0x90, 0x50, 0xD0, 0x30, 0xB0, 0x70, 0xF0, 
	0x08, 0x88, 0x48, 0xC8, 0x28, 0xA8, 0x68, 0xE8, 0x18, 0x98, 0x58, 0xD8, 0x38, 0xB8, 0x78, 0xF8, 
	0x04, 0x84, 0x44, 0xC4, 0x24, 0xA4, 0x64, 0xE4, 0x14, 0x94, 0x54, 0xD4, 0x34, 0xB4, 0x74, 0xF4, 
	0x0C, 0x8C, 0x4C, 0xCC, 0x2C, 0xAC, 0x6C, 0xEC, 0x1C, 0x9C, 0x5C, 0xDC, 0x3C, 0xBC, 0x7C, 0xFC, 
	0x02, 0x82, 0x42, 0xC2, 0x22, 0xA2, 0x62, 0xE2, 0x12, 0x92, 0x52, 0xD2, 0x32, 0xB2, 0x72, 0xF2, 
	0x0A, 0x8A, 0x4A, 0xCA, 0x2A, 0xAA, 0x6A, 0xEA, 0x1A, 0x9A, 0x5A, 0xDA, 0x3A, 0xBA, 0x7A, 0xFA,
	0x06, 0x86, 0x46, 0xC6, 0x26, 0xA6, 0x66, 0xE6, 0x16, 0x96, 0x56, 0xD6, 0x36, 0xB6, 0x76, 0xF6, 
	0x0E, 0x8E, 0x4E, 0xCE, 0x2E, 0xAE, 0x6E, 0xEE, 0x1E, 0x9E, 0x5E, 0xDE, 0x3E, 0xBE, 0x7E, 0xFE,
	0x01, 0x81, 0x41, 0xC1, 0x21, 0xA1, 0x61, 0xE1, 0x11, 0x91, 0x51, 0xD1, 0x31, 0xB1, 0x71, 0xF1,
	0x09, 0x89, 0x49, 0xC9, 0x29, 0xA9, 0x69, 0xE9, 0x19, 0x99, 0x59, 0xD9, 0x39, 0xB9, 0x79, 0xF9, 
	0x05, 0x85, 0x45, 0xC5, 0x25, 0xA5, 0x65, 0xE5, 0x15, 0x95, 0x55, 0xD5, 0x35, 0xB5, 0x75, 0xF5,
	0x0D, 0x8D, 0x4D, 0xCD, 0x2D, 0xAD, 0x6D, 0xED, 0x1D, 0x9D, 0x5D, 0xDD, 0x3D, 0xBD, 0x7D, 0xFD,
	0x03, 0x83, 0x43, 0xC3, 0x23, 0xA3, 0x63, 0xE3, 0x13, 0x93, 0x53, 0xD3, 0x33, 0xB3, 0x73, 0xF3, 
	0x0B, 0x8B, 0x4B, 0xCB, 0x2B, 0xAB, 0x6B, 0xEB, 0x1B, 0x9B, 0x5B, 0xDB, 0x3B, 0xBB, 0x7B, 0xFB,
	0x07, 0x87, 0x47, 0xC7, 0x27, 0xA7, 0x67, 0xE7, 0x17, 0x97, 0x57, 0xD7, 0x37, 0xB7, 0x77, 0xF7, 
	0x0F, 0x8F, 0x4F, 0xCF, 0x2F, 0xAF, 0x6F, 0xEF, 0x1F, 0x9F, 0x5F, 0xDF, 0x3F, 0xBF, 0x7F, 0xFF,
}

func reverseId(id uint64) (uint64) {
	result := (reverseBits[ id        & 0xff] << 56) |
		  (reverseBits[(id >>  8) & 0xff] << 48) | 
		  (reverseBits[(id >> 16) & 0xff] << 40) | 
		  (reverseBits[(id >> 24) & 0xff] << 32) | 
		  (reverseBits[(id >> 32) & 0xff] << 24) |
		  (reverseBits[(id >> 40) & 0xff] << 16) | 
		  (reverseBits[(id >> 48) & 0xff] <<  8) | 
		  (reverseBits[(id >> 56) & 0xff])
	return result >> 4
}
func reverseType(t uint64) (uint64) { return reverseBits[(t >> 60) & 0xff] << 56 }

func hash2(typeA, idA uint64) (uint64) { return idA & typeA }
func hashStrict(typeA, idA, typeZ, idZ uint64) (uint64) {
	return reverseType(typeA) & typeZ & reverseId(idA) & idZ
}
func encodeSessionId(typeA, idA, typeZ, idZ uint64) (uint64) {
	// Sort
	if typeA > typeZ { typeA, typeZ = typeZ, typeA }
	if idA > idZ { idA, idZ = idZ, idA }
	return hashStrict(idA, typeA, idZ, typeZ)
}
func decodeSessionId(sid, my uint64) (typeA, idA, typeZ, idZ uint64) { return }


func db() {
	println("[Starting] db() ......")

	// The data
	clients := make(map[uint64]*Client)
	clientTokens := make(map[string]*Client)
	rooms := make(map[uint64]*Room)
	
	roomNames := [...]string{
		"Japan", "China", "U.S.", "Russia", "U.K.",
		"Canada", "France", "German", "Korea", "AUS", "Mars~",
	}			// tmp variable
	for id, name := range roomNames {
		room := &Room{
			id		: uint64(id),
			name		: name,
			members		: make(map[uint64]*Client),
			history		: make([]*Message, 0, 20),
		}
		rooms[uint64(id)] = room
	}
	
	for query := range dbQuerys {
		var data interface{}
		fmt.Printf("[Inside query]: %v\n", query)
		
		switch (*query).action {
		case Q_NEW_CLIENT:
			clientCnt++
			client := new(Client)
			client.id = uint64(clientCnt)
			client.utype = T_USER
			client.name = fmt.Sprintf("name-%d", clientCnt)
			client.token = fmt.Sprintf("token-%d", clientCnt)
			client.rooms = make(map[uint64]*Room)
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
			for rid, room := range client.rooms {
				delete(room.members, client.id)
				msg := map[string]interface{} {
					"path": "presence",
					"action": "leave",
					"to_type": TYPE_MAP[T_ROOM],
					"to_id": rid,
					"member": map[string]interface{} {
						"oid": client.id,
					},
				}
				for _, member := range room.members {
					member.mailbox <- msg
				}
			}
			delete(clients, client.id)
			
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
			rid := uint64(query.params.(float64))
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
			rid := uint64(query.params.(float64))
			room := rooms[rid]
			history := room.history
			println("[Q_HISTORY]: len(history)", len(history))
			messages := make([](map[string]interface{}), len(history))
			for i := uint64(0); i < uint64(len(history)); i++ {
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
			rid := uint64(params["oid"].(float64))
			client := params["client"].(*Client)
			room := rooms[rid]

			for _, member := range room.members {
				println("[Join message] send to member:", member.id, member.mailbox)
				msg := map[string]interface{} {
					"path": "presence",
					"action": "join",
					"to_type": TYPE_MAP[T_ROOM],
					"to_id": rid,
					"member": map[string]interface{} {
						"oid": client.id,
						"name": client.name,
					},
				}
				member.mailbox <- msg
			}
			fmt.Printf("[Q_JOIN] rooms[rid].members: %v\n", rooms[rid].members)
			fmt.Printf("[Q_JOIN] client: %v, client.id: %v\n", client, client.id)
			client.rooms[rid] = room
			room.members[client.id] = client

		case Q_LEAVE:
			params := query.params.(map[string]interface{})
			rid := uint64(params["oid"].(float64))
			client := params["client"].(*Client)
			room := rooms[rid] 
			delete(client.rooms, rid)
			delete(room.members, client.id)

			fmt.Printf("[Q_LEAVE]: %v\n", room.members)
			for _, member := range room.members {
				println("[Leave message] send to member:", member.id, member.mailbox)
				msg := map[string]interface{} {
					"path": "presence",
					"action": "leave",
					"to_type": TYPE_MAP[T_ROOM],
					"to_id": rid,
					"member": map[string]interface{} {
						"oid": client.id,
					},
				}
				member.mailbox <- msg
			}
			
		case Q_MESSAGE:
			params := query.params.(map[string]interface{})
			rid := uint64(params["to_id"].(float64))			
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
			room.history = append(room.history, message)

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
	println(">>> Database closed !!!!!! But why???")
}


/*============================================================================
 * Actions
 *==========================================================================*/
func createClient(req map[string]interface{}, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	dbQuerys <- &Query{Q_NEW_CLIENT, nil, receiver}
	data := <-receiver
	client := data.(*Client)
	(*resp)["token"] = client.token
}

func online(req map[string]interface{}, resp *map[string]interface{}) {
	token := req["token"]
	receiver := make(chan interface{})
	dbQuerys <- &Query{Q_ONLINE, token, receiver}
	data := <-receiver

	client := data.(*Client)
	println("[Client]:", client)
	if client == nil {
		println("[online]: client is nil, reset!")
		(*resp)["reset"] = true
	} else {
		(*resp)["oid"] = (*client).id
		(*resp)["name"] = (*client).name
		(*resp)["client"] = client
	}
}

// Special Action
func offline(client *Client) {
	dbQuerys <- &Query{Q_OFFLINE, client, nil}
}

func getRooms(req map[string]interface{}, resp *map[string]interface{}) {
	receiver := make(chan interface{})
	dbQuerys <- &Query{Q_ROOMS, nil, receiver}
	rooms := <-receiver
	(*resp)["rooms"] = rooms
}

func members(req map[string]interface{}, resp *map[string]interface{}) {
	rid := req["oid"]
	receiver := make(chan interface{})
	dbQuerys <- &Query{Q_MEMBERS, rid, receiver}
	members := <-receiver
	(*resp)["oid"] = rid
	(*resp)["members"] = members
}

func history(req map[string]interface{}, resp *map[string]interface{}) {
	rid := req["oid"]
	receiver := make(chan interface{})
	dbQuerys <- &Query{Q_HISTORY, rid, receiver}
	messages := <-receiver
	(*resp)["oid"] = rid
	(*resp)["messages"] = messages
}

func join(req map[string]interface{}, resp *map[string]interface{}) {
	rid := req["oid"]
	params := map[string]interface{} {
		"client": req["client"],
		"oid": rid,
	}
	dbQuerys <- &Query{Q_JOIN, params, nil}
	(*resp)["oid"] = rid
}

func leave(req map[string]interface{}, resp *map[string]interface{}) {
	rid := req["oid"]
	params := map[string]interface{} {
		"client": req["client"],
		"oid": rid,
	}
	dbQuerys <- &Query{Q_LEAVE, params, nil}
	(*resp)["oid"] = rid
}

func message(req map[string]interface{}, resp *map[string]interface{}) () {
	to_type := req["type"]
	to_id := req["oid"]
	params := map[string]interface{} {
		"client"	: req["client"],
		"to_type"	: &to_type,
		"to_id"		: to_id, // room.id (send message to)
		"body"		: req["body"],
		"created_at"	: time.Now(),
	}
	dbQuerys <- &Query{Q_MESSAGE, params, nil}
	(*resp)["status"] = "ok"
}

func typing(req map[string]interface{}, resp *map[string]interface{}) {
	/* ::TODO */
}


// Handler for each client connection
func ChatHandler(ws *websocket.Conn) {
	fmt.Printf("[Starting] connection (%v)......\n", ws)
	
	var client *Client
	
	// Receive mssage from internal process (`func db()` actually)
	mailbox := make(chan map[string]interface{}, 2)
	go func() {
		for msg := range mailbox {
			fmt.Printf("[Mailbox]: %d, %v\n", client.id, msg)
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
			var req, resp map[string]interface{}
			// Decode request
			println("[Received]:", n, ":", string(buf[:n]))
			json.Unmarshal(buf[:n], &req)
			
			req["client"] = client
			path := req["path"].(string)
			
			// Request Dispatcher
			resp = make(map[string]interface{})
			switch path {
			case "create_client":
				createClient(req, &resp)
			case "online":
				online(req, &resp)
				if resp["client"] != nil {
					client = resp["client"].(*Client)
					client.mailbox = mailbox
				}
			case "offline":
				offline(client)
				break // Close connection
			case "rooms":
				getRooms(req, &resp) // already has `rooms`
			case "members":
				members(req, &resp)
			case "history":
				history(req, &resp)
			case "join":
				join(req, &resp)
			case "leave":
				leave(req, &resp)
			case "message":
				message(req, &resp)
			case "typing":
				typing(req, &resp)
			}

			// Encode response
			resp["path"] = path
			b, _ := json.Marshal(resp)
			ws.Write(b)
			println("[Response]:", string(b))
			
		} else {
			offline(client)
			break
		}
	}
}


func main() {

	// Init vars
	TYPE_MAP = map[uint64]string {
		T_ROOM : "room",
		T_USER : "user",
		T_VISITOR : "visitor",
	}
	dbQuerys = make(chan *Query, QUERY_BUF_SIZE)
	go db()

	go func() {
		http.Handle("/", websocket.Handler(ChatHandler))
		http.ListenAndServe(":9091", nil)
	} ()
	
	// Serve for static files
	println("[Starting] static files server......")
	public_root := filepath.Join(filepath.Dir(os.Args[0]), "public")
	http.ListenAndServe(":9090", http.FileServer(http.Dir(public_root)))
}
