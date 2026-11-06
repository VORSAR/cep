package partition_test

import (
	"testing"

	"github.com/vorsar/cep/internal/event"
	"github.com/vorsar/cep/internal/partition"
)

// TestOnArrivalReadsOneStream: an on-arrival rule reads exactly one stream (design s3-2b-1 v2 §3, MUST 2) — its
// purge argument and its admission-fed clock (owner D1) assume one.
func TestOnArrivalReadsOneStream(t *testing.T) {
	b := &oaBody{}
	if _, err := partition.NewRule(partition.Config{Inputs: []int{0, 1}, KeyTypes: []event.Type{event.TString}, TTL: 10, Body: b, OnArrival: true}); err == nil {
		t.Fatal("NewRule accepted an on-arrival rule reading two streams")
	}
	if _, err := partition.NewRule(partition.Config{Inputs: []int{0, 1}, KeyTypes: []event.Type{event.TString}, TTL: 10, Body: b}); err != nil {
		t.Fatalf("a buffered rule reading two streams: %v", err)
	}
}
