package translation

import (
	"context"
	"log"
	"testing"
)

func Test(t *testing.T) {
	text := "So why do we have rules? The goal of having rules in place is to encourage “good” behavior and discourage “bad” behavior"

	res, err := ToSentence(context.TODO(), text)

	log.Println(res, err)
}
