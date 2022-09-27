package main_test

import (
	"bytes"
	"io"
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
	stdoutPipe, err := rieCmd.StdoutPipe()
	if err != nil {
		t.Log(err)
	}
	stderrPipe, err := rieCmd.StderrPipe()
	if err != nil {
		t.Log(err)
	}

	if err := rieCmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rieCmd.Process.Signal(os.Interrupt); err != nil {
			t.Fatal(err)
		}

		stdout, err := io.ReadAll(stdoutPipe)
		if err != nil {
			t.Log(err)
		}
		t.Logf("stdout: %s", stdout)
		stderr, err := io.ReadAll(stderrPipe)
		if err != nil {
			t.Log(err)
		}
		t.Logf("stderr: %s", stderr)

		if err := rieCmd.Wait(); err != nil {
			t.Log(err)
		}
	}()

	time.Sleep(2 * time.Second)

	resp, err := http.Get("http://localhost:8080/2015-03-31/functions/function/invocations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rie response: %s", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200 OK HTTP status code. Got %d", resp.StatusCode)
	}

	// assert file
	got, err := os.ReadFile("/tmp/rie-test-journal")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantJournal) {
		t.Errorf("unexpected journal file content: got=%s, want=%s", got, wantJournal)
	}
}
