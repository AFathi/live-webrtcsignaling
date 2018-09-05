package main

import (
	"context"
	"encoding/json"
	"github.com/heytribe/go-plogger"
	"github.com/streadway/amqp"
)

type eventLogFiltersUpdate struct {
	UnitGroup string `json:"unit_group,omitempty"`
	LogFilter string `json:"log_filter"`
}

// Create logger context with prefix
var log = plogger.New()
var ctx = plogger.NewContext(context.Background(), log)

func EVUpdateLogFilters(d amqp.Delivery) {
	var err error
	var event eventLogFiltersUpdate

	jsonR := d.Body
	err = json.Unmarshal(jsonR, &event)
	if log.OnError(err, "cannot unmarshal JSON event %s", jsonR) {
		return
	}

	// only apply filters if there is not a specific unit group targeted or it's this one
	if event.UnitGroup == "" || event.UnitGroup == config.Instance.FullUnitName {
		plogger.FilterOutputs(event.LogFilter)
	}
}
