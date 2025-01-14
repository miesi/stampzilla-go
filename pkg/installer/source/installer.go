package source

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type Installer struct {
}

func NewInstaller() *Installer {
	return &Installer{}
}

func (t *Installer) Prepare() error {
	return nil
}

func (t *Installer) Install(nodes ...string) error {
	return build(nodes, false)
}

func (t *Installer) Update(nodes ...string) error {
	return build(nodes, true)
}

func build(n []string, update bool) error {
	if len(n) == 0 {
		return fmt.Errorf("you must specify which nodes to build")
	}
	// Install only specified nodes
	for _, name := range n {
		if !strings.HasPrefix(name, "stampzilla-") {
			name = "stampzilla-" + name
		}

		if !update {
			if _, err := os.Stat(filepath.Join("/home", "stampzilla", "go", "bin", name)); err == nil {
				return fmt.Errorf("%s already installed. use -u to update", name)
			}
		}

		err := GoGet(name)
		if err != nil {
			return err
		}
	}

	return nil
}

var gitDir = filepath.Join("/home", "stampzilla", "go", "src", "github.com", "stampzilla", "stampzilla-go")

func GoGet(name string) error {
	tmp := strings.Split(name, "@")
	at := "master"
	if len(tmp) == 2 {
		name = tmp[0]
		at = tmp[1]
	}
	logrus.Infof("building %s@%s", name, at)

	pwd, _ := os.Getwd()
	defer os.Chdir(pwd)
	os.Chdir("/tmp")

	// _, err = Run(gobin, "get", "-u", "-d", "github.com/stampzilla/stampzilla-go")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		_, err = Run("git", "clone", "https://github.com/stampzilla/stampzilla-go.git", gitDir)
		if err != nil {
			return err
		}
	}
	os.Chdir(gitDir)

	_, err := Run("git", "pull")
	if err != nil {
		return err
	}

	_, err = Run("git", "checkout", at)
	if err != nil {
		return err
	}

	_, err = Run("go", "mod", "download")
	if err != nil {
		return err
	}
	_, err = Run("go", getArgs(name)...)
	return err
}

func getArgs(binName string) []string {
	hash, err := Run("git", "--git-dir", filepath.Join(gitDir, ".git"), "rev-parse", "--verify", "HEAD")
	if err != nil {
		log.Fatal(err)
	}

	version := "src-" + hash[0:8] // set version to src-githash if we use src build to install

	return []string{
		"build",
		"-ldflags",
		"-X github.com/stampzilla/stampzilla-go/v2/pkg/build.Version=" + version + ` -X "github.com/stampzilla/stampzilla-go/v2/pkg/build.BuildTime=` + time.Now().Format(time.RFC3339) + `" -X github.com/stampzilla/stampzilla-go/v2/pkg/build.Commit=` + hash,
		"-o",
		filepath.Join("/home", "stampzilla", "go", "bin", binName),
		filepath.Join("/home", "stampzilla", "go", "src", "github.com", "stampzilla", "stampzilla-go", "nodes", binName),
	}
}

func Run(head string, parts ...string) (string, error) {
	var err error
	var out []byte

	cmd := exec.Command(head, parts...)

	logrus.Debugf("running command: %s %s", head, strings.Join(parts, " "))

	if current, iErr := user.Current(); iErr == nil && current.Username != "stampzilla" {
		user, err := user.Lookup("stampzilla")
		if err != nil {
			return "", err
		}
		uid, _ := strconv.Atoi(user.Uid)
		gid, _ := strconv.Atoi(user.Gid)
		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}

	cmd.Env = []string{
		"GOPATH=/home/stampzilla/go",
		"HOME=/home/stampzilla",
		"PATH=" + os.Getenv("PATH"),
	}
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error: %s : %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}
