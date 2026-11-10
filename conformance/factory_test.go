package conformance_test

import (
	"context"
	"testing"
	"time"

	"github.com/vorsar/cep"
	cepio "github.com/vorsar/cep/io"
	"github.com/vorsar/cep/io/inmemory"
)

// corpusFactory binds every @source and @sink annotation of a corpus file to an io/inmemory connector, whatever its
// type, so a file is never refused for a connector this release does not build (S3.3-1's design decision 5: an
// unbuilt connector deploys in tests through a Go binding; design s3-3-2 §7).
type corpusFactory struct{}

func (corpusFactory) Source(string, map[string]string) (cepio.Source, error) {
	return inmemory.NewSource(0), nil
}

func (corpusFactory) Sink(string, map[string]string) (cepio.Sink, error) {
	return &inmemory.Sink{}, nil
}

// TestCorpusFactoryBindsEveryAnnotation: a plain rule whose streams carry http annotations — refused by the default
// factory until S5.1 — deploys through corpusFactory.
func TestCorpusFactoryBindsEveryAnnotation(t *testing.T) {
	const src = `language cep/0.1;

@source(type = 'http', path = '/in')
stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

@sink(type = 'http', url = 'http://localhost/out')
stream Out (k string, ts timestamp);

rule Forward
  intent 'Forward every event.'
  from S
  select k
  insert into Out;
`
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // the ceiling; nothing here waits
	defer cancel()
	r, err := cep.New(cep.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close(ctx) }()
	if _, err := r.Deploy(ctx, "annotated", src); err == nil {
		t.Fatal("the default factory deployed an http annotation; the witness is gone")
	}
	if _, err := r.Deploy(ctx, "annotated", src, cep.WithConnectors(corpusFactory{})); err != nil {
		t.Fatalf("Deploy through corpusFactory: %v", err)
	}
}
