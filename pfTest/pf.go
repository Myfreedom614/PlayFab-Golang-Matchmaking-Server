package pfTest

import (
	"encoding/json"
	"fmt"
	"time"

	playfab "github.com/dgkanatsios/playfabsdk-go/sdk"
	auth "github.com/dgkanatsios/playfabsdk-go/sdk/authentication"
	mps "github.com/dgkanatsios/playfabsdk-go/sdk/multiplayer"
)

type AttrDataObject struct {
	Latency []LatencyItem
}

type LatencyItem struct {
	region  string
	latency float32
}

func main() {
	settings := playfab.NewSettingsWithDefaultOptions(titleId)

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
	entityToken := res1.EntityToken
	//entity := res1.Entity
	fmt.Printf("Title EntityToken: %s\n", entityToken)

	// CreateServerMatchmakingTicket
	attrData := `{
		"Latency": [
			{
				"region": "ChinaEast2",
				"latency": 70
			},
			{
				"region": "ChinaNorth2",
				"latency": 80
			}
		]
	}`
	var attrDataObj interface{}
	attrDataObj = composeJsonObj((attrData))

	if attrDataObj == nil {
		handleFail(fmt.Sprint("Matchmaking ticket attr format error"))
	}

	mpsTicketReqData := &mps.CreateServerMatchmakingTicketRequestModel{
		Members: []mps.MatchmakingPlayerModel{
			{
				Attributes: &mps.MatchmakingPlayerAttributesModel{
					DataObject: attrDataObj,
				},
				Entity: &mps.EntityKeyModel{
					Id:   "3A19654FEB889FE4",
					Type: "title_player_account",
				},
			},
		},
		GiveUpAfterSeconds: 300,
		QueueName:          queueName,
	}

	fmt.Println(prettyPrint(mpsTicketReqData))

	res, err := mps.CreateServerMatchmakingTicket(settings, mpsTicketReqData, entityToken)

	if err != nil {
		handleFail(fmt.Sprintf("CreateServerMatchmakingTicket Error: %s\n", err.Error()))
		return
	}

	ticketId := res.TicketId
	fmt.Printf("CreateServerMatchmakingTicket OK, ticket id: %s\n", ticketId)

	// GetMatchmakingTicket
	matchId := ""
	stopped := false
	ticker := time.NewTicker(6 * time.Second) //Up to 10 times per minute
	for !stopped {
		select {
		case <-ticker.C:
			mpsGetTicketReqData := &mps.GetMatchmakingTicketRequestModel{
				TicketId:     ticketId,
				QueueName:    queueName,
				EscapeObject: false,
			}

			fmt.Println(prettyPrint(mpsGetTicketReqData))

			res2, err2 := mps.GetMatchmakingTicket(settings, mpsGetTicketReqData, entityToken)

			if err2 != nil {
				handleFail(fmt.Sprintf("GetMatchmakingTicket Error: %s\n", err2.Error()))
				return
			}

			fmt.Printf("Match Status: %s\n", res2.Status)

			if res2.Status == "Matched" {
				matchId = res2.MatchId
				fmt.Printf("MatchId: %s\n", res2.MatchId)
				ticker.Stop()
				stopped = true
			}
		}
	}

	// GetMatch
	mpsGetMatchReqData := &mps.GetMatchRequestModel{
		MatchId:      matchId,
		QueueName:    queueName,
		EscapeObject: false,
	}

	fmt.Println(prettyPrint(mpsGetMatchReqData))

	res3, err3 := mps.GetMatch(settings, mpsGetMatchReqData, entityToken)

	if err3 != nil {
		handleFail(fmt.Sprintf("GetMatch Error: %s\n", err3.Error()))
		return
	}

	fmt.Printf("GetMatch-Members: %s\n", prettyPrint(res3.Members))
	fmt.Printf("GetMatch-RegionPreferences: %s\n", prettyPrint(res3.RegionPreferences))

}

func prettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
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

func handleFail(msg string) {
	panic(msg)
}
