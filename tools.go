package toolkit

import (
	"crypto/rand"
	"log"
)

const randomStringSource = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_+"

// Tools is the type used to instantiate this module. Any variable of this type will have access to all the methods 
// with the receiver *Tools
type Tools struct {}

// RandomString returns a string of randomn characters of length n, using randomStringSource 
// as the source for teh characters of the string
func (t * Tools) RandomString(n int) string {
	s, r := make([]rune, n), []rune(randomStringSource)

	for i := range s {
		p, err := rand.Prime(rand.Reader, len(r))
		if err != nil {
			log.Fatal("Unexpected error ", err)
		}		
		x, y := p.Uint64(), uint64(len(r))
		s[i] = r[x%y]
	}

	return string(s)
}