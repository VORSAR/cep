package cep_test

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// Example_embed is the owner's check (S3.3's card; QA's T40): example 09 deployed, fed two rows and stopped, the
// one alert printed as its canonical JSON.
func Example_embed() {
	ctx := context.Background()
	src, _ := os.ReadFile("docs/language/examples/09-params-and-quoting.cep")
	rt, err := cep.New(cep.Config{Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	defer rt.Close(ctx)
	sink := &inmemory.Sink{}
	app, err := rt.Deploy(ctx, "09", string(src), cep.WithSink("RiskAlerts", sink))
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	if err := app.Start(ctx); err != nil {
		fmt.Println("error:", err)
		return
	}
	in, _ := app.Input("agent-events")
	at := time.Date(2026, 1, 1, 0, 0, 1, 0, time.UTC).UnixMilli()
	for _, risk := range []float64{0.9, 0.5} {
		err := in.SendRow(ctx, "", func(w cepio.RowWriter) error {
			w.SetString(w.AttrIndex("agentId"), "a1")
			w.SetFloat(w.AttrIndex("risk"), risk)
			w.SetString(w.AttrIndex("env"), "prod")
			w.SetTimestamp(w.AttrIndex("ts"), at)
			return nil
		})
		if err != nil {
			fmt.Println("error:", err)
			return
		}
		at += 1000
	}
	if _, err := app.Stop(ctx); err != nil {
		fmt.Println("error:", err)
		return
	}
	for _, rec := range sink.Drain() {
		fmt.Println(string(rec.Canonical))
	}
	// Output:
	// {"attrs":{"agentId":"a1","scaled":1,"ts":"2026-01-01T00:00:01.000Z"},"stream":"RiskAlerts","ts":"2026-01-01T00:00:01.000Z"}
}
