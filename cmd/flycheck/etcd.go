package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fly-examples/fly-etcd/pkg/flyetcd"
)

func CheckEtcd(ctx context.Context, client *flyetcd.Client, passed []string, failed []error) ([]string, []error) {

	msg, err := checkAlarms(ctx, client)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	return passed, failed
}

func checkAlarms(ctx context.Context, client *flyetcd.Client) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, (5 * time.Second))
	resp, err := client.AlarmList(ctx)
	cancel()
	if err != nil {
		return "", err
	}
	var alarms []string
	for _, alarm := range resp.Alarms {
		alarms = append(alarms, alarm.Alarm.String())
	}

	if len(alarms) > 0 {
		return "", fmt.Errorf("alarm(s) active: %s", strings.Join(alarms, ", "))
	}
	return "no alarms active", nil
}
