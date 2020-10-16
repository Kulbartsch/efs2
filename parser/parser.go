// Package parser is an Efs2file parser. This package provides the ability to read an Efs2file and convert it into Efs2 tasks.
package parser

import (
	"bufio"
	"fmt"
	"github.com/madflojo/efs2/ssh"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parse will open and read the specified Efs2file. It will then return a slice of tasks built from the file.
func Parse(f string) ([]ssh.Task, error) {
	var tasks []ssh.Task
	var s *bufio.Scanner

	// Open file or stdin if given "-"
	if f == "-" {
		s = bufio.NewScanner(os.Stdin)
	} else {
		fh, err := os.Open(f)
		if err != nil {
			return nil, fmt.Errorf("could not read Efs2file - %s", err)
		}
		defer fh.Close()
		s = bufio.NewScanner(fh)
	}

	// Matches all RUN instructions
	isRun := regexp.MustCompile(`^RUN .*$`)
	// Matches an older RUN instruction syntax
	isOldRun := regexp.MustCompile(`^RUN (CMD|SCRIPT) .*$`)
	// Matches PUT instructions
	isPut := regexp.MustCompile(`^PUT .* \d{3,4}$`)
	// Matches Comments
	isComment := regexp.MustCompile(`^#.*`)

	lc := 0
	for s.Scan() {
		lc = lc + 1

		l := strings.TrimSpace(s.Text())
		c := strings.Split(l, " ")

		if !isRun.MatchString(l) && !isOldRun.MatchString(l) && !isPut.MatchString(l) && !isComment.MatchString(l) {
			return tasks, fmt.Errorf("Unable to parse Efs2file line %s", l)
		}

		t := ssh.Task{
			Task:    l,
			Command: ssh.Command{},
			File:    ssh.File{},
		}

		// Match current RUN instruction syntax
		if isRun.MatchString(l) && !isOldRun.MatchString(l) {
			t.Command.Cmd = strings.Join(c[1:], " ")
			tasks = append(tasks, t)
		}

		// Match older RUN instruction syntax
		if isOldRun.MatchString(l) {
			if c[1] == "CMD" {
				t.Command.Cmd = strings.Join(c[2:], " ")
			}

			if c[1] == "SCRIPT" {
				dest := "/tmp/" + TmpFn()
				t.Command.Cmd = dest + "; rm " + dest
				t.File.Source = c[2]
				t.File.Destination = dest
				t.File.Mode = os.FileMode(int(0700))
			}

			tasks = append(tasks, t)
		}

		// Match PUT instructions
		if isPut.MatchString(l) {

			p := strings.Split(l, " ")
			if len(p) != 4 {
				return tasks, fmt.Errorf("PUT definition on line %d is incorrect", lc)
			}

			t.File.Source = p[1]
			t.File.Destination = p[2]

			m, err := strconv.ParseUint(p[3], 8, 32)
			if err != nil {
				return tasks, fmt.Errorf("could not convert mode value to integer on line %d - %s", lc, p[3])
			}
			t.File.Mode = os.FileMode(m)

			tasks = append(tasks, t)
		}
	}
	if err := s.Err(); err != nil {
		return tasks, fmt.Errorf("error parsing Efs2file - %s", err)
	}

	return tasks, nil
}

// TmpFn will generate a temporary filename
func TmpFn() string {
	// Snagged from ioutil.TempFile
	r := uint32(time.Now().UnixNano() + int64(os.Getpid()))
	r = r*1664525 + 1013904223
	return strconv.Itoa(int(1e9 + r%1e9))[1:]
}