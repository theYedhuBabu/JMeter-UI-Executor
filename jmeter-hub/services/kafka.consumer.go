package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"
	"time"

	"jmeter-hub/net"

	"github.com/segmentio/kafka-go"
)

/*
Internal metric used by your hub / DB
THIS STRUCT REMAINS UNCHANGED
*/
type KafkaMetric struct {
	Timestamp    int64  `json:"timestamp"`
	TestName     string `json:"test_name"`
	ThreadName   string `json:"thread_name"`
	Label        string `json:"label"`
	Elapsed      int    `json:"elapsed"`
	ResponseCode int    `json:"response_code"`
	Latency      int    `json:"latency"`
	Success      bool   `json:"success"`
	AllThreads   int    `json:"all_threads"`
	AgentId	     string `json:"agent_id"`
}

/*
RawKafkaMetric matches EXACTLY what Kafka sends.
This prevents schema mismatches.
*/
type RawKafkaMetric struct {
	Timestamp    int64  `json:"timestamp"`
	TestName     string `json:"test_name"`
	ThreadName   string `json:"thread_name"`
	Label        string `json:"label"`
	Elapsed      int    `json:"elapsed"`
	ResponseCode string `json:"response_code"` // string from Kafka
	Latency      int    `json:"latency"`
	Success      bool   `json:"success"`
	AllThreads   int    `json:"all_threads"`
	AgentId	     string `json:"agent_id"`
}

// StartKafkaMetricsPipeline starts both consumer + aggregator
func StartKafkaMetricsPipeline(hub *net.Hub, brokers []string, topic string) {

	metricChan := make(chan KafkaMetric, 10000)

	startKafkaConsumer(metricChan, brokers, topic)
	startMetricAggregator(hub, metricChan)
}

func startKafkaConsumer(metricChan chan KafkaMetric, brokers []string, topic string) {

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: "hub-metrics-consumer",
	})

	go func() {

		for {

			msg, err := reader.ReadMessage(context.Background())
			if err != nil {
				slog.Error("Kafka read failed", "error", err)
				continue
			}

			// Step 1: Parse raw Kafka metric
			var raw RawKafkaMetric

			err = json.Unmarshal(msg.Value, &raw)
			if err != nil {
				slog.Error("Failed to parse Kafka metric", "error", err, "raw", string(msg.Value))
				continue
			}

			// Step 2: Convert response_code string -> int
			code, err := strconv.Atoi(raw.ResponseCode)
			if err != nil {
				code = 0 // fallback for non-HTTP responses
			}

			// Step 3: Convert to internal struct
			metric := KafkaMetric{
				Timestamp:    raw.Timestamp,
				TestName:     raw.TestName,
				ThreadName:   raw.ThreadName,
				Label:        raw.Label,
				Elapsed:      raw.Elapsed,
				ResponseCode: code,
				Latency:      raw.Latency,
				Success:      raw.Success,
				AllThreads:   raw.AllThreads,
			}

			metricChan <- metric
		}
	}()
}

func startMetricAggregator(hub *net.Hub, metricChan chan KafkaMetric) {

	go func() {

		var requests int
		var errors int
		var threads int

		ticker := time.NewTicker(1 * time.Second)

		for {

			select {

			case m := <-metricChan:

				requests++

				if !m.Success {
					errors++
				}

				threads = m.AllThreads

			case <-ticker.C:

				payload := struct {
					Type string `json:"type"`
					Data struct {
						Requests int `json:"requests"`
						Errors   int `json:"errors"`
						Threads  int `json:"threads"`
					} `json:"data"`
				}{
					Type: "metric",
				}

				payload.Data.Requests = requests
				payload.Data.Errors = errors
				payload.Data.Threads = threads

				bytes, err := json.Marshal(payload)
				if err == nil {
					hub.Broadcast <- bytes
				}

				// reset for next second
				requests = 0
				errors = 0
			}
		}
	}()
}