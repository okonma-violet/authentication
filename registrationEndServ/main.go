package main

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/tarantool/go-tarantool"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type configs struct {
	tarantoolConn *tarantool.Connection
	mongoConn     *mongo.Client
	mongoColl     *mongo.Collection
}

var ctx = context.Background()
var emailRegex = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

type Tuple struct {
	Login string
	Check bool
}

type UserInfo struct {
	Login   string `bson:"_id"`
	Name    string `bson:"name"`
	Surname string `bson:"surname"`
}

func (cfg *configs) handler(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		fmt.Println(err) //todo
		return
	}

	mailUid := r.FormValue("guid")
	userName := r.FormValue("name")
	userSurname := r.FormValue("surname")
	if len(userName) < 2 || len(userSurname) < 2 || len(userName) > 30 || len(userSurname) > 30 {
		fmt.Println("check naming") //todo
		return
	}

	userPass := r.FormValue("password")
	userPassLen := len(userPass)
	if userPassLen < 5 {
		fmt.Println("check pass") //todo
		return
	}

	//TODO get from limbo
	userMail := w.Header().Get("X-Foo")
	userMailHash, err := GetMD5(userMail)
	if err != nil {
		fmt.Println(err) //todo
		return
	}
	userPassHash, err := GetMD5(userPass)
	if err != nil {
		fmt.Println(err) //todo
		return
	}

	userInfo := &UserInfo{Login: userMailHash, Name: userName, Surname: userSurname}

	_, err = cfg.mongoColl.InsertOne(ctx, userInfo)
	if err != nil {
		fmt.Println(err) //todo
		return
	}

	_, err = cfg.tarantoolConn.Insert("main", []interface{}{userMailHash, userPassHash})
	if err != nil {
		fmt.Println(err) //todo
		return
	}

	// TODO: SEND CONGRATS TO EMAIL HERE

	client := &http.Client{}
	respCookieGen, err := client.Get("http://127.0.0.1:8089?l=" + userMailHash)
	if err != nil {
		fmt.Println(err) //todo
		return
	}
	if respCookieGen.StatusCode != http.StatusOK {
		fmt.Println("bad resp from cookieGen") //todo
		return
	}
	respCookies := respCookieGen.Cookies()
	if err != nil {
		fmt.Println(err) //todo
		return
	}
	http.SetCookie(w, respCookies[0])
	//add return

}

func main() {
	connTrntl, err := tarantool.Connect("localhost:3301", tarantool.Opts{
		// User:          "admin",
		// Pass:          "password",
		Timeout:       500 * time.Millisecond,
		Reconnect:     1 * time.Second,
		MaxReconnects: 4,
	})
	if err != nil {
		fmt.Println(err) //todo
		return
	}
	defer func() {
		if connTrntl.Close() != nil {
			fmt.Println(err) //todo
			return
		}
	}()

	connMng, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		fmt.Println(err) //todo
		return
	}
	defer func() {
		if connMng.Disconnect(ctx) != nil {
			fmt.Println(err)
		}
	}()

	collectionMng := connMng.Database("main").Collection("users")

	cfg := *&configs{tarantoolConn: connTrntl, mongoConn: connMng, mongoColl: collectionMng}

	http.HandleFunc("/", cfg.handler)
	log.Fatal(http.ListenAndServe(":8085", nil))
}

func GetMD5(str string) (string, error) {
	hash := md5.New()
	_, err := hash.Write([]byte(str))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
