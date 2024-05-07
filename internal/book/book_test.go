package book

import (
	"fmt"
	"testing"
)

func TestParseId(testing *testing.T) {
	expected := "foo/bar"

	url := fmt.Sprintf("https://online.fliphtml5.com/%s", expected)

	actual, err := ParseId(url)
	if err != nil {
		testing.Fatalf("unexpected error: %v", err)
	}

	if actual != expected {
		testing.Fatalf("expected %s, got %s", expected, actual)
	}
}
