/*
Package app is the main runtime package for Efs2. This package holds all of the logging and task execution controls.
*/
package app

import (
	"efs2/config"
	"efs2/parser"
	"efs2/ssh"
	"fmt"
	"github.com/fatih/color"
	"github.com/howeyc/gopass"
	"regexp"
	"sync"
)

// Encrypted Key Error
var isPassErr = regexp.MustCompile(`.*[decode encrypted|protected].*$`)

// Has a port defined
var hasPort = regexp.MustCompile(`.*:\d*`)

// Run is the primary execution function for this application.
func Run(cfg config.Config) error {
	var clientCfg ssh.Config
	var err error

	// If Password is set
	if cfg.Password != "" {
		clientCfg = ssh.Config{
			Password: cfg.Password,
		}
		cfg.KeyFile = ""
	}

	if cfg.Password == "" {
		// If no Password is set
		clientCfg, err = ssh.ReadKeyFile(cfg.KeyFile, cfg.Passphrase)
		if err != nil {
			if !isPassErr.MatchString(err.Error()) {
				return fmt.Errorf("Unable to obtain Key Passphrase - %s", err)
			}
			color.White("Enter Private Key Passphrase: ")
			cfg.Passphrase, err = gopass.GetPasswd()
			if err != nil {
				return fmt.Errorf("Unable to obtain Key Passphrase - %s", err)
			}
			clientCfg, err = ssh.ReadKeyFile(cfg.KeyFile, cfg.Passphrase)
			if err != nil {
				return fmt.Errorf("Unable to read keyfile - %s", err)
			}
		}
	}

	// Check if Efs2file is defined
	if cfg.Efs2File == "" {
		cfg.Efs2File = "./Efs2file"
	}

	// Setup User
	clientCfg.User = cfg.User

	// Loudness
	if cfg.Verbose && !cfg.Quiet {
		color.Yellow("SSH User: %s", cfg.User)
		color.Yellow("Key Path: %s", cfg.KeyFile)
		color.Yellow("Efs2file Path: %s", cfg.Efs2File)
	}

	// Parse Efs2file
	tasks, err := parser.Parse(cfg.Efs2File)
	if err != nil {
		return fmt.Errorf("Unable to parse Efs2file - %s", err)
	}

	// Fixup Hosts
	cfg.Hosts = fixUpHosts(cfg.Hosts, cfg.Port)

	// Execute
	var wg sync.WaitGroup
	var errCount int
	for _, h := range cfg.Hosts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := clientCfg
			c.Host = h
			sh, err := ssh.Dial(c)
			if err != nil {
				errCount = errCount + 1
				if !cfg.Quiet {
					color.Red("%s: Error connecting to host - %s", h, err)
				}
				return
			}
			for i, t := range tasks {
				if !cfg.Quiet {
					color.Blue("%s: Executing Task %d - %s", h, i, t.Task)
				}
				if cfg.DryRun {
					continue
				}
				if t.File.Source != "" {
					err := sh.Put(t.File)
					if err != nil {
						errCount = errCount + 1
						if !cfg.Quiet {
							color.Red("%s: Error uploading file - %s", h, err)
						}
						return
					}
					if !cfg.Quiet {
						color.Blue("%s: File upload successful", h)
					}
				}
				if t.Command.Cmd != "" {
					r, err := sh.Run(t.Command)
					if err != nil {
						errCount = errCount + 1
						if !cfg.Quiet {
							color.Red("%s: Error executing command - %s", h, err)
						}
						return
					}
					if !cfg.Quiet {
						color.Blue("%s: %s", h, r)
					}
				}
			}

		}()
		if !cfg.Parallel {
			wg.Wait()
		}
	}
	wg.Wait()

	if errCount > 0 {
		return fmt.Errorf("Execution failed with %d errors", errCount)
	}
	return nil
}

func fixUpHosts(hosts []string, port string) []string {
	// Fixup Hosts
	var hh []string
	for _, h := range hosts {
		if hasPort.MatchString(h) {
			hh = append(hh, h)
			continue
		}
		if port == "" {
			hh = append(hh, h+":22")
			continue
		}
		hh = append(hh, h+":"+port)
	}
	return hh
}
