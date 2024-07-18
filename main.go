package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	//"github.com/uber-go/zap"
	// go get -u go.uber.org/zap
)

var timeoutDur time.Duration = 60 * 1000000000
var _KASversion string = "v0.01"
var _KASport string = "9085"

type MessengerType int64

const (
	Unknown MessengerType = iota - 1
	Telegram
	Matrix
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func Reader(conn *websocket.Conn, timeout time.Duration) {
	lastResponse := time.Now()
	var isAlive bool
	isAlive = true
	isAlivePong := true
	var messageType_ int
	//var messenger *MessengerTelegram
	var messenger IMessengerSender

	go func() { // isAlive *bool
		var d time.Duration = 5 * 1000000000
		//var d time.Duration = timeout / 2
		for {
			// err := conn.WriteMessage(websocket.PingMessage, []byte("keepalive"))
			// if err != nil {
			// 	return
			// }

			//time.Sleep(timeout / 2)
			time.Sleep(d)

			if time.Since(lastResponse) > timeout {
				if messenger != nil {
					errStr := messenger.SendOffline()
					if errStr != "" {
						respToClient := MakeClientResponse(errStr)
						if err := conn.WriteMessage(messageType_, []byte(respToClient)); err != nil {
							// log.Println("BBBBB")
							// log.Println(err)
							//return
						}
					}
				}
				if isAlive {
					// 2023-Jan-31 07:12:06UTC,KeepAliveServer: CatalogsChecker is offline❌ (no message in 60s)
					isAlivePong = false
					log.Println("connection alive - closing; timeout!!!")
					conn.Close()
				} else {
					log.Println("connection already closed; timeout!!!")
					//conn.Close()
				}
				return
			}
		}
	}() // &isAlive

	for { // Authentification
		messageType, p, err := conn.ReadMessage()
		messageType_ = messageType
		//isOk, errStr, messenger_ := AuthSetter(string(p))
		errStr, _, messenger_ := AuthSetter(string(p))

		if errStr == "" { // all alright
			messenger = messenger_
			//errStr = "auth successful"
			if messenger_ == nil {
				errStr = "messenger is null"
				//isOk = false
			}
		}

		if err != nil {
			log.Println(err)
			isAlive = false
			return
		}
		lastResponse = time.Now()

		respToClient := MakeClientResponse(errStr)
		if err := conn.WriteMessage(messageType, []byte(respToClient)); err != nil { // errStr
			log.Println(err)
			isAlive = false
			return
		}
		if errStr != "" {
			// close connection!!!
			isAlive = false
			return
		}
		break
	}

	errStr := messenger.SendOnline()
	if errStr != "" {
		isAlive = false
		respToClient := MakeClientResponse(errStr)
		if err := conn.WriteMessage(messageType_, []byte(respToClient)); err != nil {
			// log.Println("BBBBB")
			// log.Println(err)
			//return
		}
		conn.Close()
		return
	}

	// resp, err := http.Get((messenger).httpUrl + messenger.MakeStrOnline()) // "New client connected"
	// if err != nil {
	// 	fmt.Println("ERRORRRRRR")
	// 	log.Fatalln(err)
	// }
	// fmt.Println(resp)
	// fmt.Println(resp.StatusCode) // 200 ideally

	for {
		// read in a message
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			if isAlivePong {
				// 2023-Jan-31 07:12:06UTC,KeepAliveServer: CatalogsChecker is offline❌ (no message in 60s)
				// send mes
				log.Println("connection alive - isAlivePong")
			}
			//log.Println("AAAAA")
			log.Println(err)
			conn.Close()
			break
			//isAlive = false
			//return
		}
		// conn.SetPongHandler(func(msg string) error {
		// 	lastResponse = time.Now()
		// 	return nil
		// })
		//keepAlive(conn, timeout)

		lastResponse = time.Now()
		if len(p) == 0 {
			log.Println("ping")
			continue
		}
		mesFromClient := string(p)
		log.Println(mesFromClient)

		errStr := messenger.SendMessage(&mesFromClient)
		if errStr != "" {
			isAlive = false
			respToClient := MakeClientResponse(errStr)
			if err := conn.WriteMessage(messageType, []byte(respToClient)); err != nil {
				// log.Println("BBBBB")
				// log.Println(err)
				//return
			}
			conn.Close()
			break
		}

		// if err := conn.WriteMessage(messageType, p); err != nil {
		// 	log.Println("BBBBB")
		// 	log.Println(err)
		// 	break
		// 	//isAlive = false
		// 	//return
		// }

	}
	isAlive = false
}

// func homePage(w http.ResponseWriter, r *http.Request) {
// 	fmt.Fprintf(w, "Home Page")
// }

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	// upgrade this connection to a WebSocket
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
	}

	log.Println("Client Connected")
	// err = ws.WriteMessage(1, []byte("Hi Client!")) // &#91;]
	// if err != nil {
	// 	log.Println(err)
	// }
	//keepAlive(ws, d)
	Reader(ws, timeoutDur)
}

func setupRoutes() {
	//http.HandleFunc("/", homePage)
	http.HandleFunc("/ws", wsEndpoint)
}

// func (s MessengerType) String() string {
// 	switch s {
// 	case Telegram:
// 		return "telegram"
// 	case Matrix:
// 		return "matrix"
// 	}
// 	return "unknown"
// }

func StrToMessenger(s string) MessengerType {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "")
	switch s {
	case "telegram":
		return Telegram
	case "matrix":
		return Matrix
	}
	return Unknown
}

func AuthSetter(authJson string) (string, MessengerType, IMessengerSender) { // *MessengerTelegram // interface{}
	if !json.Valid([]byte(authJson)) {
		return "invalid json", Unknown, nil
	}
	var authMap map[string]any
	json.Unmarshal([]byte(authJson), &authMap)

	mesType := Unknown
	messengerTypeStr, ok := authMap["messengerType"].(string)
	if !ok {
		return "missing mandatory fields", Unknown, nil
	} else {
		mesType = StrToMessenger(messengerTypeStr)
	}

	if mesType == Unknown {
		return "unknown messenger type", Unknown, nil
	}

	scriptName, ok := authMap["scriptName"].(string)
	if !ok {
		return "missing mandatory fields", Unknown, nil
	}
	if strings.ReplaceAll(scriptName, " ", "") == "" {
		return "scriptName is empty field", Unknown, nil
	}

	loginInfo, ok := authMap["loginInfo"].(map[string]any) // any
	if !ok {
		return "missing mandatory fields", Unknown, nil
	}

	// If Telegram!!!!
	if mesType == Telegram {
		botID, ok := loginInfo["botID"].(string)
		if !ok {
			return "missing mandatory fields", Unknown, nil
		}
		chatID, ok := loginInfo["chatID"].(string)
		if !ok {
			return "missing mandatory fields", Unknown, nil
		}
		//var msger *TgMessenger
		msger := new(MessengerTelegram)
		msger.scriptName = scriptName
		msger.mesType = mesType
		msger.chatID = chatID
		msger.botID = botID
		msger.httpUrl = "https://api.telegram.org/bot" + (*msger).botID + "/sendMessage?chat_id=" + (*msger).chatID + "&text="
		return "", Unknown, msger
	} else if mesType == Matrix {
		federation, ok := loginInfo["federation"].(string)
		if !ok {
			return "missing mandatory fields", Unknown, nil
		}
		room, ok := loginInfo["room"].(string)
		if !ok {
			return "missing mandatory fields", Unknown, nil
		}
		token, ok := loginInfo["token"].(string)
		if !ok {
			return "missing mandatory fields", Unknown, nil
		}
		//var msger *TgMessenger
		msger := new(MessengerMatrix)
		msger.scriptName = scriptName
		msger.mesType = mesType
		msger.federation = federation
		msger.room_alias = room
		msger.token = token

		MatrixRoomConvert(&msger.room_alias, &msger.federation)

		return "", Unknown, msger
	} else {
		return "unknown mesType", Unknown, nil
	}
}

func MakeClientResponse(in string) string {
	err_str := "{\"err\":\"" + in + "\"}"
	return err_str
}

func GetTimeForResponse() string {
	// 2023-Jan-31 04:11:14UTC
	//dt := time.Now().UTC()
	return time.Now().UTC().Format("2006-Jan-02 15:04:05") //+ "UTC"
}

// type TypeA struct {
// 	a string
// 	b string
// }
// type TypeB struct {
// 	a string
// 	c string
// }
// func maker(s string) interface{} {
// 	if s == "a" {
// 		msger := new(TypeA)
// 		(*msger).a = "abc"
// 		(*msger).b = "abcd"
// 		return msger
// 	} else if s == "b" {
// 		msger := new(TypeB)
// 		(*msger).a = "abc2"
// 		(*msger).c = "abcd2"
// 		return msger
// 	} else {
// 		return nil
// 	}
// }
// type iType interface {
// 	common(s string) string
// }
// func (t *Type) common(s string) string {
// 	fmt.Println("hi", s, (*t).a)
// 	return "1"
// }
// type Type struct {
// 	a string
// }
// type Type1 struct {
// 	Type
// 	b string
// }
// type Type2 struct {
// 	Type
// 	c string
// }

type TgMessenger struct {
	mesType    MessengerType
	scriptName string
	botID      string
	chatID     string
	httpUrl    string
}

////

type IMessenger interface {
	MakeStrOnline() string
	MakeStrMessage(mes *string) string
	MakeStrOffline() string
}

func (m *Messenger) MakeStrOnline() string {
	//2023-Jan-31 04:11:14UTC,KeepAliveServer: CatalogsChecker is online✅
	//var scriptName string
	var res bytes.Buffer
	res.WriteString(GetTimeForResponse())
	res.WriteString("UTC,KeepAliveServer: ")
	res.WriteString(m.scriptName)
	res.WriteString(" is online✅")
	return res.String()
}
func (m *Messenger) MakeStrMessage(mes *string) string {
	//2023-Feb-01 00:15:03UTC,KeepAliveServer: Message from CatalogsChecker: Merge_start
	//var scriptName string
	var res bytes.Buffer
	res.WriteString(GetTimeForResponse())
	res.WriteString("UTC,KeepAliveServer: Message from ")
	res.WriteString(m.scriptName)
	res.WriteString(": ")
	res.WriteString(*mes)
	return res.String()
}
func (m *Messenger) MakeStrOffline() string {
	//2023-Jan-31 07:12:06UTC,KeepAliveServer: CatalogsChecker is offline❌ (no message in 60s)
	//var scriptName string
	var res bytes.Buffer
	res.WriteString(GetTimeForResponse())
	res.WriteString("UTC,KeepAliveServer: ")
	res.WriteString(m.scriptName)
	res.WriteString(" is offline❌ (no message in 60s)")
	return res.String()
}

type IMessengerSender interface {
	SendOnline() string
	SendMessage(mes *string) string
	SendOffline() string
}

func (m *MessengerTelegram) SendOnline() string {
	resp, err := http.Get(m.httpUrl + m.MakeStrOnline())
	if err != nil {
		//fmt.Println("ERRORRRRRR")
		//log.Fatalln(err)
		return err.Error()
	}
	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}
	return ""
}
func (m *MessengerTelegram) SendMessage(mes *string) string {
	resp, err := http.Get(m.httpUrl + m.MakeStrMessage(mes))
	if err != nil {
		//fmt.Println("ERRORRRRRR")
		//log.Fatalln(err)
		return err.Error()
	}
	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}
	return ""
}
func (m *MessengerTelegram) SendOffline() string {
	resp, err := http.Get(m.httpUrl + m.MakeStrOffline())
	if err != nil {
		//fmt.Println("ERRORRRRRR")
		//log.Fatalln(err)
		return err.Error()
	}
	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}
	return ""
}

func (m *MessengerMatrix) SendOnline() string {
	// GET https://matrix.dexex.ru/_matrix/client/r0/directory/room/<room_alias>

	get_room_id := "https://" + m.federation + "/_matrix/client/r0/directory/room/" + m.room_alias
	resp1, err1 := http.Get(get_room_id)
	if err1 != nil {
		return err1.Error()
	}
	if resp1.StatusCode != 200 {
		return strconv.Itoa(resp1.StatusCode)
	}
	var res1 map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&res1)
	room1, ok1 := res1["room_id"]
	if !ok1 {
		return "response doesn't have room_id in response GET req to get room_id by room_alias"
	}
	m.room_id = room1.(string)
	m.httpUrl = "https://" + m.federation + "/_matrix/client/r0/rooms/" + m.room_id + "/send/m.room.message?access_token=" + m.token

	post_join := "https://" + m.federation + "/_matrix/client/r0/rooms/" + m.room_id + "/join?access_token=" + m.token
	values2 := map[string]string{}
	json_data2, _ := json.Marshal(values2)
	resp2, _ := http.Post(post_join, "application/json",
		bytes.NewBuffer(json_data2))
	if resp2.StatusCode != 200 {
		return strconv.Itoa(resp2.StatusCode)
	}
	var res2 map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&res2)
	_, ok2 := res2["room_id"]
	if !ok2 {
		return "response doesn't have room_id in response POST req joining room"
	}
	m.room_id = room1.(string)

	values := map[string]string{"msgtype": "m.text", "body": m.MakeStrOnline()}
	json_data, err := json.Marshal(values)
	if err != nil {
		//log.Fatal(err)
		return err.Error()
	}
	resp, err := http.Post(m.httpUrl, "application/json",
		bytes.NewBuffer(json_data))

	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	_, ok := res["event_id"]
	if !ok {
		return "response doesn't have event_id"
	}
	return ""
}
func (m *MessengerMatrix) SendOffline() string {
	// #{"msgtype":"m.text", "body":"test2"}

	values := map[string]string{"msgtype": "m.text", "body": m.MakeStrOffline()}
	json_data, err := json.Marshal(values)
	if err != nil {
		//log.Fatal(err)
		return err.Error()
	}
	resp, err := http.Post(m.httpUrl, "application/json",
		bytes.NewBuffer(json_data))

	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	_, ok := res["event_id"]
	if !ok {
		return "response doesn't have event_id"
	}
	return ""
}
func (m *MessengerMatrix) SendMessage(mes *string) string {
	// #{"msgtype":"m.text", "body":"test2"}
	values := map[string]string{"msgtype": "m.text", "body": m.MakeStrMessage(mes)}
	json_data, err := json.Marshal(values)
	if err != nil {
		//log.Fatal(err)
		return err.Error()
	}
	resp, err := http.Post(m.httpUrl, "application/json",
		bytes.NewBuffer(json_data))

	if resp.StatusCode != 200 {
		return strconv.Itoa(resp.StatusCode)
	}

	var res map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&res)
	_, ok := res["event_id"]
	if !ok {
		return "response doesn't have event_id"
	}
	return ""
}

type Messenger struct {
	mesType    MessengerType
	scriptName string
}

type MessengerTelegram struct {
	Messenger
	botID   string
	chatID  string
	httpUrl string
}

type MessengerMatrix struct {
	Messenger
	federation string
	//user       string
	//password   string
	room_alias string
	room_id    string
	token      string
	httpUrl    string
}

func MatrixRoomConvert(room *string, federation *string) { // modifies room to matrix needence
	//room := "abcd"
	//var room_matrix string
	if len(*room) > 0 {
		if (*room)[0] == '#' {
			*room = strings.ReplaceAll(*room, "#", "%23")
		} else {
			//var p string
			//p := *room
			//*room = ""
			*room = "%23" + *room
		}
	}
	if *federation != "" && !strings.Contains(*room, *federation) {
		*room = *room + ":" + *federation
	}
	//return room_matrix
}

func main() {
	// f, err := os.OpenFile("testfile", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// logger := log.New(f, "", 0)
	// log.SetFlags(log.Lmicroseconds)
	// logger.Output(2, "wow")

	// //fmt.Println("hello!")
	// return

	// s := "abcd"
	// if len(s) > 0 {
	// 	if s[0] == '#' {
	// 		s = strings.ReplaceAll(s, "#", "%23")
	// 	} else {
	// 		p := s
	// 		s = ""
	// 		s = "%23" + p
	// 	}
	// }
	// feder := "matrix.dexex.ru"
	// s := "abcd:matrix.dexex.ru"
	// MatrixRoomConvert(&s, &feder)
	// fmt.Println(s)
	// s = "#abcd:matrix.dexex.ru"
	// MatrixRoomConvert(&s, &feder)
	// fmt.Println(s)
	// return

	// t := new(Type1)
	// t.a = "aaa"
	// fmt.Printf(t.common("abc9"))
	// return

	// var msger *TypeB
	// var msger_ (interface{})
	// msger = maker("b").(*TypeB)
	// fmt.Println((*msger).a)

	// msger_ = msger
	// fmt.Println(msger_.a)
	// return

	//cur_time := GetTimeForResponse()
	//log.Println(cur_time) // 2023/02/10 22:10:25
	//fmt.Println(cur_time)

	// pigeon := make(map[string]string)
	// pigeon["err"] = "ok"
	// pigeon["success"] = "1"
	// data, _ := json.Marshal(pigeon)
	// fmt.Println(string(data))

	// err_str := "invalid data"
	// err_str = "{\"err\":\"" + err_str + "\"}"

	//fmt.Println(MakeAuthResponse("Муся"))
	//return

	// var s string
	// s = "matrix"
	// fmt.Println(StrToMessenger(s))

	// birdJson := `{"birds":{"pigeon":"likes to perch on rocks","eagle":"bird of prey"},"animals":"none"}`

	// var result map[string]any
	// json.Unmarshal([]byte(birdJson), &result)

	// birds := result["birds"].(map[string]any)
	// b := birds["pigeon"]
	// fmt.Println(b)

	// animals, ok := result["animalss"].(string)
	// if !ok { //!json.Valid([]byte(birdJson)) {
	// 	// handle the error here
	// 	fmt.Println("invalid JSON string11:")
	// 	return
	// } else {
	// 	fmt.Println(animals)
	// }

	// for key, value := range birds {
	// 	fmt.Println(key, value.(string))
	// }
	// return

	//ws://localhost:8080/ws
	fmt.Println("KeepAliveServerUniversal", _KASversion, "started")
	setupRoutes()
	log.Fatal(http.ListenAndServe(":"+_KASport, nil))
	//log.Fatal(http.ListenAndServeTLS(":8080", nil))
}
