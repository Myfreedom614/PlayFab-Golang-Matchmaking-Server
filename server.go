package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	playfab "github.com/dgkanatsios/playfabsdk-go/sdk"
	auth "github.com/dgkanatsios/playfabsdk-go/sdk/authentication"
	mps "github.com/dgkanatsios/playfabsdk-go/sdk/multiplayer"
)

type MatchRequest struct {
	DataObject         interface{}
	TitleAccountId     string
	QueueName          string
	GiveUpAfterSeconds int32
}

type MatchInfo struct {
	MatchId   string
	QueueName string
}

var logger *log.Logger
var settings *playfab.Settings
var entityToken string = ""

func InitLog() {
	logFile, err := os.OpenFile("./output.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Panic("Open log file failed")
	}
	logger = log.New(logFile, "", log.Ldate|log.Ltime|log.Lshortfile)
}

func InitPF() {
	settings = playfab.NewSettingsWithDefaultOptions(titleId)

	// ENTITY API - Get title level Entity Token
	r1 := &auth.GetEntityTokenRequestModel{}
	res1, err := auth.GetEntityToken(settings, r1, "", "", developerSecretKey)
	if err != nil {
		handleFail(fmt.Sprintf("GetEntityToken should not return err, Error:%s", err.Error()))
	}
	if res1.Entity.Id == "" {
		handleFail("entityId should be defined")
	}
	if res1.Entity.Type == "" {
		handleFail("entityType should be defined")
	}
	entityToken = res1.EntityToken
	//entity := res1.Entity

	commonOutput(fmt.Sprintf("Title EntityToken: %s\n", entityToken))
}

func GameyeRequest(url string, method string, postData []byte) {
	req, err := http.NewRequest(method, gameyeUrl+url, bytes.NewBuffer(postData))
	req.Header.Add("Authorization", "Bearer "+gameyeToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		handleFail(fmt.Sprintf("GameyeAPIRequest %s Error: %s\n", url, err.Error()))
	}
	defer resp.Body.Close()

	commonOutput(fmt.Sprintf("GameyeAPIRequest %s response Status: %s, Headers: %s \n", url, resp.Status, resp.Header))
	body, _ := ioutil.ReadAll(resp.Body)
	commonOutput(fmt.Sprintf("GameyeAPIRequest %s response Body: %s\n", url, string(body)))
}

func GameyePostRequest(url string, postData []byte) {
	GameyeRequest(url, http.MethodPost, postData)
}

func GameyeGetRequest(url string) {
	GameyeRequest(url, http.MethodGet, []byte(""))
}

func main() {
	InitLog()

	// ENTITY API - Get title level Entity Token
	InitPF()

	//
	http.HandleFunc("/CreateSinglePlayerTicket", func(res http.ResponseWriter, req *http.Request) {
		//Get Request Data
		var data MatchRequest
		err := json.NewDecoder(req.Body).Decode(&data)
		if err != nil || data.QueueName == "" || data.TitleAccountId == "" {
			res.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(res, `{"Status":"BadRequest"}`)
			//http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		if data.GiveUpAfterSeconds == 0 {
			data.GiveUpAfterSeconds = 300 //Default 300 seconds
		}

		fmt.Println(prettyPrint(data.DataObject))

		// CreateServerMatchmakingTicket
		mpsTicketReqData := &mps.CreateServerMatchmakingTicketRequestModel{
			Members: []mps.MatchmakingPlayerModel{
				{
					Attributes: &mps.MatchmakingPlayerAttributesModel{
						DataObject: data.DataObject,
					},
					Entity: &mps.EntityKeyModel{
						Id:   data.TitleAccountId,
						Type: "title_player_account",
					},
				},
			},
			GiveUpAfterSeconds: 300,
			QueueName:          data.QueueName,
		}

		commonOutput(fmt.Sprintln(prettyPrint(mpsTicketReqData)))

		res1, err1 := mps.CreateServerMatchmakingTicket(settings, mpsTicketReqData, entityToken)

		if err1 != nil {
			handleFail(fmt.Sprintf("CreateServerMatchmakingTicket Error: %s\n", err1.Error()))
			return
		}

		ticketId := res1.TicketId
		commonOutput(fmt.Sprintf("CreateServerMatchmakingTicket OK, ticket id: %s\n", ticketId))

		// get response headers
		header := res.Header()
		// set content type header
		header.Set("Content-Type", "application/json")

		// set status header
		res.WriteHeader(http.StatusOK)
		// respond with a JSON string
		fmt.Fprintf(res, `{"Status":"OK", "TicketId": %s}`, ticketId)

		//Then, move to /matchfound API to get Match info and notify Gameye
	})

	http.HandleFunc("/matchfound", func(res http.ResponseWriter, req *http.Request) {
		var data MatchInfo
		err := json.NewDecoder(req.Body).Decode(&data)
		if err != nil || data.MatchId == "" || data.QueueName == "" {
			res.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(res, `{"Status":"BadRequest"}`)
			//http.Error(res, err.Error(), http.StatusBadRequest)
			return
		}

		// get response headers
		header := res.Header()
		// set content type header
		header.Set("Content-Type", "application/json")

		// set status header
		res.WriteHeader(http.StatusOK)
		// respond with a JSON string
		fmt.Fprintf(res, `{"Status":"OK", "MatchInfo": %s}`, prettyPrint(data))

		//Continue to do the following jobs
		if entityToken == "" {
			handleFail(fmt.Sprintf("EntityToken is empty, MatchInfo: %s\n", prettyPrint(data)))
			return
		}

		go func() {
			// GetMatch
			mpsGetMatchReqData := &mps.GetMatchRequestModel{
				MatchId:                data.MatchId,
				QueueName:              data.QueueName,
				EscapeObject:           false,
				ReturnMemberAttributes: true,
			}

			commonOutput(fmt.Sprintln(prettyPrint(mpsGetMatchReqData)))

			res3, err3 := mps.GetMatch(settings, mpsGetMatchReqData, entityToken)

			if err3 != nil {
				handleFail(fmt.Sprintf("GetMatch Error: %s\n", err3.Error()))
				return
			}

			commonOutput(fmt.Sprintf("GetMatch-Members: %s\n", prettyPrint(res3.Members)))
			commonOutput(fmt.Sprintf("GetMatch-RegionPreferences: %s\n", prettyPrint(res3.RegionPreferences)))

			// var jsonStr = []byte(`{
			// 	"matchKey": "my-awesome-match",
			// 	"gameKey": "shooter-game",
			// 	"locationKeys": [
			// 	  "eu-west"
			// 	],
			// 	"templateKey": "deathmatch",
			// 	"config": {
			// 	  "map": "de_dust"
			// 	},
			// 	"endCallbackUrl": "https://mybackend/matchid",
			// 	"restart": true,
			// 	"sortAdvantages": [
			// 	  "price"
			// 	]
			//   }`)
			// GameyePostRequest("command/start-match", jsonStr)
			//GameyeGetRequest("query/match")
		}()
	})

	fmt.Printf("Starting server at port 9000\n")
	if err := http.ListenAndServe(":9000", nil); err != nil {
		logger.Fatal(err)
		//fmt.Println(err)
	}
}

func commonOutput(msg string) {
	if printCMDLog {
		fmt.Print(msg)
	}
	logger.Print(msg)
}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func handleFail(msg string) {
	//To-Do: Handle EntityToken expired, then refresh Token: call InitPF(). 400/500 errorcode
	logger.Panic(msg)
}

func composeJsonObj(s string) interface{} {
	var jsonObj interface{}
	err := json.Unmarshal([]byte(s), &jsonObj)

	if err != nil {
		fmt.Printf("ComposeJsonObj Error:%s\n", err.Error())
		return nil
	}

	return jsonObj
}
