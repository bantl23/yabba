package cmd

import (
	"os"
	"testing"
)

func TestVersionCmd_DefaultValues(t *testing.T) {
	// Version and Hash must have their sentinel defaults when no -ldflags
	// injection is performed (i.e., during go test).
	if Version != "v0.0.0" {
		t.Errorf("Version = %q, want %q", Version, "v0.0.0")
	}
	if Hash != "n/a" {
		t.Errorf("Hash = %q, want %q", Hash, "n/a")
	}
}

func TestVersionCmd_Execute(t *testing.T) {
	// Capture stdout via an os.Pipe so that fmt.Printf output from
	// versionCmd.Run is intercepted.
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	rootCmd.SetArgs([]string{"version"})
	execErr := Execute()

	w.Close()
	os.Stdout = origStdout

	buf := make([]byte, 256)
	n, _ := r.Read(buf)
	r.Close()
	got := string(buf[:n])

	if execErr != nil {
		t.Fatalf("Execute() returned error: %v", execErr)
	}

	want := "v0.0.0 (n/a)\n"
	if got != want {
		t.Errorf("version output = %q, want %q", got, want)
	}

	// Reset args so other tests are not affected.
	rootCmd.SetArgs(nil)
}
