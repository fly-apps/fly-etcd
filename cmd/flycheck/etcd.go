package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fly-apps/fly-etcd/internal/flyetcd"
)

func CheckEtcd(ctx context.Context, client *flyetcd.Client, passed []string, failed []error) ([]string, []error) {

	msg, err := checkAlarms(ctx, client)
	if err != nil {
		failed = append(failed, err)
	} else {
		passed = append(passed, msg)
	}

	msg, err = checkConnectivity(ctx, client)
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

func checkConnectivity(ctx context.Context, client *flyetcd.Client) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, (5 * time.Second))
	start := time.Now()
	// get a random key. As long as we can get the response without an error, the
	// endpoint is health.
	_, err := client.Get(ctx, "health")
	cancel()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("healthy: true, took: %v", time.Since(start)), nil
}
