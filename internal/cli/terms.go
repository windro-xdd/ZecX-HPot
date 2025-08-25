package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const termsURL = "https://github.com/ZecurX-Projects/ZecX-Honeypot/blob/main/TERMS.md" // Placeholder URL

// AcceptTerms prompts the user to accept the terms and conditions.
// It returns true if the user types "yes", and false otherwise.
func AcceptTerms() bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("By using ZecX-Honeypot, you agree to our Terms and Conditions for legal and ethical use.\n")
	fmt.Printf("Please review them at: %s\n", termsURL)
	fmt.Print("Do you accept these terms? (yes/no): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		return false
	}

	return strings.TrimSpace(strings.ToLower(input)) == "yes"
}
