package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gorilla/websocket"

	"github.com/go-playground/validator/v10"
)

var (
	upGrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	validate *validator.Validate
)

// WsWallte ...
func WsWallte(c *gin.Context) {
	ws, err := upGrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	for {
		mt, message, err := ws.ReadMessage()
		if err != nil {
			log.Println(err)
			break
		}

		defaultMessage := []byte("MessageType Not In Case.")

		var rd RedisData

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
			switch true {
			case bytes.Index(message, []byte(postDeductCmd)) > 0:
				rd.PostData = &PostDeduct{}
			case bytes.Index(message, []byte(postSaveCmd)) > 0:
				rd.PostData = &PostSave{}
			default:
				ws.WriteMessage(mt, []byte("Not Allow Action"))
				break
			}

			if err := json.Unmarshal([]byte(message), &rd.PostData); err != nil {
				ws.WriteMessage(mt, []byte(err.Error()))
				break
			}

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

			_, err = WalletEntry(&rd)
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
	r := GetRedisClientInstance()

	accountStart := 201
	accountEnd := 501

	hashMap := make(map[string]interface{})
	hashMap[redisHashWalletKey] = 100000
	hashMap[redisHashlastChangeKey] = time.Now()
	hashMap[redisHashlastGameKey] = 1

	for i := accountStart; i < accountEnd; i++ {
		u := BuildRedisDataWithDelimiter(redisUserPerfix, "agent001", strconv.Itoa(i))
		r.UserWalletHMSet(&RedisData{userKey: u, hashMap: hashMap})
	}

	c.JSON(http.StatusOK, gin.H{"succes": true})
}

// DeductWalletController ...
func DeductWalletController(c *gin.Context) {
	var rd RedisData

	rd.PostData = &PostDeduct{}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := WalletEntry(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd})
}

// SaveWalletController ...
func SaveWalletController(c *gin.Context) {
	var rd RedisData

	rd.PostData = &PostSave{}

	if err := c.ShouldBindJSON(&rd.PostData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status, err := WalletEntry(&rd)
	if err != nil {
		c.JSON(status, gin.H{"succes": false, "data": err.Error()})
	}

	c.JSON(http.StatusOK, gin.H{"succes": true, "data": &rd})
}

func produce(ch chan<- string, p *PostData) {
	time.Sleep(2 * time.Second)
	ch <- p.Agent + p.User

	close(ch)
}

func chinSelectCaseFunc(c *gin.Context) {
	var p PostData
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
	var p PostData
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
