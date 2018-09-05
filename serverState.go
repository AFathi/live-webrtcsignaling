package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	plogger "github.com/heytribe/go-plogger"
	"github.com/heytribe/live-rabbitmqlib"
)

type statsResponse struct {
	Success bool            `json:"s"`
	Error   string          `json:"e,omitempty"`
	Data    json.RawMessage `json:"d,omitempty"`
}

type statsResponseData struct {
	FullUnitName string `json:"full_unit_name"`
	LogFilters   string `json:"log_filters"`
	Rooms        *Rooms `json:"rooms"`
}

func generateServerStateJson() (jsonStr string) {
	statsResponse := statsResponse{true, "", nil}

	statsResponseData := statsResponseData{config.Instance.FullUnitName, plogger.CurrentFilters(), rooms}
	dataJson, err := json.Marshal(statsResponseData)
	if err != nil {
		statsResponse.Success = false
		statsResponse.Error = err.Error()
	} else {
		statsResponse.Data = dataJson
	}

	//respJson, err := json.MarshalIndent(statsResponse, "", "  ")
	respJson, err := json.Marshal(statsResponse)
	if err != nil {
		jsonStr = fmt.Sprintf(`{"s": false, "e": "%v"}`, err)
	} else {
		jsonStr = string(respJson)
	}

	return
}

// unused anymore
func SendStatePeriodicallyToAMQP(ctx context.Context, rmq liverabbitmq.Rmq) {
	log := plogger.FromContextSafe(ctx)
	ticker := time.NewTicker(1000 * time.Millisecond)

	for range ticker.C {
		err := rmq.EventMessageSend(liverabbitmq.LiveBackendEvents, liverabbitmq.LiveAdminEventServerStateRK, []byte(generateServerStateJson()))
		if err != nil {
			log.Errorf("error sending serverState message %s", err)
		}
	}
}

func httpStateController(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, generateServerStateJson())
}

func getServerStateContext() (ctx context.Context) {
	ctx = context.Background()
	log := plogger.FromContextSafe(ctx)
	log.Tag("serverState").Prefix("marshalJson")
	ctx = plogger.NewContext(ctx, log)
	return
}
