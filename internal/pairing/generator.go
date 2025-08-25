package pairing

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// Word lists for generating human-readable codes.
// In a real application, these lists would be much larger.
var adjectives = []string{
	"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
	"indigo", "juliett", "kilo", "lima", "mike", "november", "oscar", "papa",
	"quebec", "romeo", "sierra", "tango", "uniform", "victor", "whiskey", "xray",
	"yankee", "zulu",
}

var nouns = []string{
	"phoenix", "griffin", "dragon", "hydra", "sphinx", "pegasus", "siren",
	"cyclops", "golem", "chimera", "kraken", "basilisk", "wyvern", "minotaur",
	"cerberus", "unicorn",
}

// GenerateCode creates a cryptographically secure, random, yet human-readable code.
func GenerateCode() (string, error) {
	adj, err := randomWord(adjectives)
	if err != nil {
		return "", fmt.Errorf("could not generate adjective: %w", err)
	}

	noun, err := randomWord(nouns)
	if err != nil {
		return "", fmt.Errorf("could not generate noun: %w", err)
	}

	// Generate a random number between 1000 and 9999
	num, err := rand.Int(rand.Reader, big.NewInt(9000))
	if err != nil {
		return "", fmt.Errorf("could not generate random number: %w", err)
	}
	num.Add(num, big.NewInt(1000)) // Add 1000 to get it in the range 1000-9999

	return fmt.Sprintf("%s-%s-%d", adj, noun, num.Int64()), nil
}

func randomWord(wordList []string) (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(wordList))))
	if err != nil {
		return "", err
	}
	return wordList[n.Int64()], nil
}
