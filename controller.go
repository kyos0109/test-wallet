package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"

	"github.com/go-playground/validator/v10"

	"github.com/kyos0109/test-wallet/config"
	"github.com/kyos0109/test-wallet/database"
	"github.com/kyos0109/test-wallet/kredis"
	"github.com/kyos0109/test-wallet/modules"
	"github.com/kyos0109/test-wallet/wallet"
)

const (
	verifyFirstString = "happy"
	postDeductCmd     = "deduct"
	postStoreCmd      = "store"
)

var (
	upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	validate     *validator.Validate
	verifyLength = len(verifyFirstString)
)

// CommandFunc ...
type CommandFunc struct {
	Open   bool
	Detail interface{}
	Fn     func(rd *modules.RedisData) (int, error)
}

// WsWallte ...
func WsWallte(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	CmdFunc := map[string]CommandFunc{
		postDeductCmd: {Fn: wallet.Entry, Detail: &modules.PostDeductv2{}, Open: true},
		postStoreCmd:  {Fn: wallet.Entry, Detail: &modules.PostStorev2{}, Open: true},
	}

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		defaultMessage := []byte("MessageType Not In Case.")

		var rd modules.RedisData

		switch mt {
		case websocket.TextMessage:
			if string(message) == "ping" {
				message = []byte("pong")
			} else {
				err := ws.WriteMessage(mt, []byte(message))
				if err != nil {
					break
				}
			}
		case websocket.BinaryMessage:
			if len(message) < verifyLength {
				err := ws.WriteMessage(mt, []byte("Verify Error."))
				if err != nil {
					break
				}
			}

			if string(message[:verifyLength]) != verifyFirstString {
				ws.WriteMessage(mt, []byte("Not Match"))
				break
			}

			jsonMsg := message[verifyLength:]
			if err := json.Unmarshal(jsonMsg, &rd.PostData); err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

			cmd, ok := CmdFunc[rd.PostData.Action]
			if !ok {
				ws.WriteMessage(mt, []byte("Not Allow Action"))
				break
			}

			rd.PostData.Detail = cmd.Detail
			if err := json.Unmarshal(jsonMsg, &rd.PostData); err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

			rd.PostData.ClientIP = c.ClientIP()
			reqURL := c.Request.Host + c.Request.URL.String()
			rd.PostData.RequestURL = reqURL
			rd.PostData.RequestProto = c.Request.Proto

			validate = validator.New()
			err := validate.Struct(rd.PostData)

			if err != nil {
				if _, ok := err.(*validator.InvalidValidationError); ok {
					log.Println(err)
					break
				}

				vErrs := err.(validator.ValidationErrors)
				ws.WriteMessage(mt, []byte(vErrs.Error()))
				break
			}

			_, err = cmd.Fn(&rd)
			if err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

			json, err := json.Marshal(&rd)
			if err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

			err = ws.WriteMessage(mt, json)
			if err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

		default:
			err := ws.WriteMessage(mt, []byte(defaultMessage))
			if err != nil {
				break
			}
		}
	}
}

// CreateRedisData fake data
func CreateRedisData(c *gin.Context) {
	r := kredis.GetRedisClientInstance()

	accountStart := 201
	accountEnd := 501

	hashMap := make(map[string]interface{})
	hashMap[config.RedisHashWalletKey] = 100000
	hashMap[config.RedisHashlastChangeKey] = time.Now()
	hashMap[config.RedisHashlastGameKey] = 1

	for i := accountStart; i < accountEnd; i++ {
		u := wallet.BuildRedisDataWithDelimiter(config.RedisUserPrefix, "100", strconv.Itoa(i))
		r.UserWalletHMSet(&modules.RedisData{UserKey: u, HashMap: hashMap})
	}

	c.JSON(http.StatusOK, gin.H{"succes": true})
}

var happy = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

func randStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = happy[rand.Intn(len(happy))]
	}
	return string(b)
}

func randAgentID(max int, ids []string) string {
	return ids[rand.Intn(max)]
}

// CreateDBData ...
func CreateDBData(c *gin.Context) {
	accountStart := 0
	accountEnd := 5001
	agnetNums := 1
	agentIDs := []string{}
	fakeAgent := []modules.Agent{}

	for a := 0; a < agnetNums; a++ {
		agentIDs = append(agentIDs, randStringRunes(8))
		fakeAgent = append(fakeAgent, modules.Agent{ID: agentIDs[a], Key: randStringRunes(128), PrivateKey: []byte(randStringRunes(512)), Status: true, UpdateAt: time.Now()})
	}

	fakeUsers := []modules.User{}

	for i := accountStart; i < accountEnd; i++ {
		fakeUsers = append(fakeUsers, modules.User{Name: randStringRunes(32), Status: true, AgentID: randAgentID(agnetNums, agentIDs), UpdateAt: time.Now()})
	}
	pg := database.GetDBInstance()
	pg.CreateFakceAgent(fakeAgent)
	pg.CreateFakceUser(fakeUsers)

	users := []modules.User{}
	wallets := []modules.Wallet{}

	r := pg.Conn().WithContext(ctx).Find(&users, modules.User{Status: true})
	if r.Error != nil {
		log.Fatalln(r.Error)
	}

	for _, v := range users {
		wallets = append(wallets, modules.Wallet{UserID: v.ID, Amount: 100000.00, Currency: "TWD", UpdateAt: time.Now()})
	}
	pg.CreateFakceWallet(wallets)

	c.JSON(http.StatusOK, gin.H{"succes": true})
}

// DeductWalletController ...
func DeductWalletController(c *gin.Context) {
	var rd modules.RedisData

	rd.PostData = &modules.PostDatav2{
		ClientIP:     c.ClientIP(),
		RequestURL:   c.Request.Host + c.Request.URL.String(),
		RequestProto: c.Request.Proto,
		Detail:       &modules.PostDeductv2{},
	}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.Entry(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd.Order})
	return
}

// StoreWalletController ...
func StoreWalletController(c *gin.Context) {
	var rd modules.RedisData

	rd.PostData = &modules.PostDatav2{
		ClientIP:     c.ClientIP(),
		RequestProto: c.Request.Proto,
		RequestURL:   c.Request.Host + c.Request.URL.String(),
		Detail:       &modules.PostStorev2{},
	}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.Entry(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd.Order})
	return
}

func produce(ch chan<- string, p *modules.PostData) {
	time.Sleep(2 * time.Second)
	ch <- p.Agent + p.User

	close(ch)
}

func chinSelectCaseFunc(c *gin.Context) {
	var p modules.PostData
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	//I keep the channels in this slice, and want to "loop" over them in the select statemnt
	var chans = []chan string{}
	ch := make(chan string)
	chans = append(chans, ch)
	go produce(ch, &p)

	cases := make([]reflect.SelectCase, len(chans))
	for i, ch := range chans {
		cases[i] = reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ch)}
	}

	remaining := len(cases)
	for remaining > 0 {
		chosen, value, ok := reflect.Select(cases)
		if !ok {
			// The chosen channel has been closed, so zero out the channel to disable the case
			cases[chosen].Chan = reflect.ValueOf(nil)
			remaining = remaining - 1
			continue
		}

		fmt.Printf("Read from channel %#v and received %s\n", chans[chosen], value.String())
	}
	c.JSON(http.StatusOK, gin.H{"succes": true, "data": p})
}

func chinSelectFunc(c *gin.Context) {
	var p modules.PostData
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ch := make(chan string)
	go produce(ch, &p)
	// fmt.Printf("%#v \n", ch)
	<-ch

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": p})
}

// DeductWalletControllerDB ...
func DeductWalletControllerDB(c *gin.Context) {
	var p modules.PostDatav2

	p.ClientIP = c.ClientIP()
	p.RequestURL = c.Request.Host + c.Request.URL.String()
	p.RequestProto = c.Request.Proto
	p.Detail = &modules.PostDeductv2{}

	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.EntryDB(&p)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &p})
}

// StoreWalletControllerDB ...
func StoreWalletControllerDB(c *gin.Context) {
	var p modules.PostDatav2

	p.ClientIP = c.ClientIP()
	p.RequestURL = c.Request.Host + c.Request.URL.String()
	p.RequestProto = c.Request.Proto
	p.Detail = &modules.PostStorev2{}

	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.EntryDB(&p)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &p})
}

// EchoResponse ...
func EchoResponse(c *gin.Context) {
	var p modules.PostDatav2

	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"echo": &p})
	return
}

// DeductSingleWalletController ...
func DeductSingleWalletController(c *gin.Context) {
	var rd modules.RedisData

	rd.PostData = &modules.PostDatav2{
		ClientIP:     c.ClientIP(),
		RequestURL:   c.Request.Host + c.Request.URL.String(),
		RequestProto: c.Request.Proto,
		Detail:       &modules.PostDeductv2{},
	}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.EntrySingle(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd.Order})
	return
}

// StoreSingleWalletController ...
func StoreSingleWalletController(c *gin.Context) {
	var rd modules.RedisData

	rd.PostData = &modules.PostDatav2{
		ClientIP:     c.ClientIP(),
		RequestURL:   c.Request.Host + c.Request.URL.String(),
		RequestProto: c.Request.Proto,
		Detail:       &modules.PostStorev2{},
	}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := wallet.EntrySingle(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd.Order})
	return
}
