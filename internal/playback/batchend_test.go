package playback_test

// TestBatchEndRoundTrip (design s3-3-1 §2.9, MUST 11): the recorder's BatchEnd writes the batch_end record the reader
// already defines; a recording with one BatchEnd(0, "b1") reads back as Record{Kind: RecBatchEnd, Stream: 0, Batch: "b1"}.

import (
	"bytes"
	"testing"

	"github.com/vorsar/cep/internal/playback"
)

const beSrc = `language cep/0.1;

stream S (k string, ts timestamp)
  time ts watermark bounded 1 min;

stream O (k string, ts timestamp);

rule Fwd
  intent 'Forward every row.'
  from S
  select k
  insert into O;
`

func TestBatchEndRoundTrip(t *testing.T) {
	t.Parallel()
	p := pbBuild(t, "batchend.cep", beSrc)
	var buf bytes.Buffer
	rec := playback.NewRecorder(&buf, p)
	if err := rec.Header(ldT0); err != nil {
		t.Fatal(err)
	}
	if err := rec.BatchEnd(0, "b1"); err != nil {
		t.Fatalf("BatchEnd: %v", err)
	}
	if err := rec.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := rec.Flush(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"batch_end":"b1"`)) || !bytes.Contains(buf.Bytes(), []byte(`"stream":"S"`)) {
		t.Errorf("recording:\n%s\nwant the batch_end record {\"batch_end\":\"b1\",\"stream\":\"S\"}", buf.String())
	}
	rd, err := playback.NewReader(bytes.NewReader(buf.Bytes()), p)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	if r, err := rd.Next(); err != nil || r.Kind != playback.RecHeader {
		t.Fatalf("first record %+v, %v; want the header", r, err)
	}
	r, err := rd.Next()
	if err != nil {
		t.Fatalf("second record: %v", err)
	}
	if r.Kind != playback.RecBatchEnd || r.Stream != 0 || r.Batch != "b1" {
		t.Errorf("second record %+v; want Kind RecBatchEnd, Stream 0, Batch b1", r)
	}
	if r, err := rd.Next(); err != nil || r.Kind != playback.RecStop {
		t.Errorf("third record %+v, %v; want the stop", r, err)
	}
}
