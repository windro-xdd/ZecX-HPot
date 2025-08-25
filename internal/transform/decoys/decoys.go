package decoys

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// decoy represents a file or directory to be created.
type decoy struct {
	Path    string
	Content string
	IsDir   bool
}

// See https://github.com/fabacab/awesome-cyber-security-blueteam/blob/master/README.md#honeypots--honeynets
var decoyStructure = []decoy{
	// Fake user homes
	{Path: "/home/alice", IsDir: true},
	{Path: "/home/bob", IsDir: true},
	{Path: "/home/charlie", IsDir: true},

	// Fake bash histories with common commands
	{Path: "/home/alice/.bash_history", Content: "ls -la\ncd /var/www\nnano index.html\nexit\n"},
	{Path: "/home/bob/.bash_history", Content: "sudo apt-get update\nps aux\nssh admin@10.0.0.5\n"},
	{Path: "/home/charlie/.zsh_history", Content: "git clone https://github.com/some/repo.git\ncd repo\nmake\n"},

	// Fake application directories and configs
	{Path: "/var/www/html", IsDir: true},
	{Path: "/var/www/html/index.html", Content: "<h1>Welcome</h1><p>Under construction.</p>"},
	{Path: "/etc/nginx/sites-available", IsDir: true},
	{Path: "/etc/nginx/sites-available/default", Content: "server {\n\tlisten 80;\n\troot /var/www/html;\n}"},

	// Fake logs
	{Path: "/var/log/apache2", IsDir: true},
	{Path: "/var/log/apache2/access.log", Content: "127.0.0.1 - - [10/Oct/2022:13:55:36 +0000] \"GET / HTTP/1.1\" 200 151\n"},

	// Fake credentials
	{Path: "/root/.ssh", IsDir: true},
	{Path: "/root/.aws", IsDir: true},
	{Path: "/root/.ssh/id_rsa", Content: "-----BEGIN RSA PRIVATE KEY-----\nMIIEogIBAAKCAQEAr... (fake key)\n-----END RSA PRIVATE KEY-----\n"},
	{Path: "/root/.aws/credentials", Content: "[default]\naws_access_key_id = AKIAIOSFODNN7EXAMPLE\naws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\n"},
}

// Seed creates a believable decoy filesystem environment.
func Seed() error {
	log.Println("Seeding decoy environment with real files and directories...")
	for _, d := range decoyStructure {
		// Ensure the base directory for the file exists
		dir := d.Path
		if !d.IsDir {
			dir = filepath.Dir(d.Path)
		}

		// Create the directory structure
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("Failed to create decoy directory %s: %v", dir, err)
			// Continue even if one fails
			continue
		}

		// If it's a directory, we're done with this entry
		if d.IsDir {
			log.Printf("Created decoy directory: %s", d.Path)
			continue
		}

		// Create and write content to the file
		if err := os.WriteFile(d.Path, []byte(d.Content), 0644); err != nil {
			log.Printf("Failed to create decoy file %s: %v", d.Path, err)
			continue
		}
		log.Printf("Created decoy file: %s", d.Path)
	}
	fmt.Println("Decoy environment seeded.")
	return nil
}

// Clean removes all decoy files and directories.
func Clean() error {
	log.Println("Cleaning decoy environment...")
	// It's safer to remove the top-level directories we created.
	topLevelDirs := []string{"/home/alice", "/home/bob", "/home/charlie", "/var/www", "/etc/nginx", "/var/log/apache2", "/root/.ssh", "/root/.aws"}
	for _, dir := range topLevelDirs {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("Failed to remove decoy directory %s: %v", dir, err)
			// Continue cleanup even if one fails
		} else {
			log.Printf("Removed decoy directory tree: %s", dir)
		}
	}
	return nil
}
