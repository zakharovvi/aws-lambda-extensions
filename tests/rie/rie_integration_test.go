package main_test

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

var wantJournal = []byte(`extension.Init
function.HandleRequest
extension.HandleInvokeEvent
`)

func TestRIE(t *testing.T) {
	rieCmd := exec.Command("/tmp/aws-lambda-rie", "./rie")
	if err := rieCmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rieCmd.Process.Signal(os.Interrupt); err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/2015-03-31/functions/function/invocation")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// assert file
	got, err := os.ReadFile("/tmp/rie-test-journal")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantJournal) {
		t.Errorf("unexpected journal file content: got=%s, want=%s", got, wantJournal)
	}
}
