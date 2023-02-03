package toolkit

import "testing"

func TestToolsRandomString(t *testing.T) {
	var testTools Tools

	n := 10
	s := testTools.RandomString(n)
	if len(s) != n {
		t.Errorf("Unexpected length. Expected: %d, Got: %d", n, len(s))
	} 
}
