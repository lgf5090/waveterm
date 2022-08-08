// Copyright 2022 Dashborg Inc
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package base

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

const HomeVarName = "HOME"
const DefaultMShellHome = "~/.mshell"
const DefaultMShellName = "mshell"
const MShellPathVarName = "MSHELL_PATH"
const MShellHomeVarName = "MSHELL_HOME"
const SSHCommandVarName = "SSH_COMMAND"
const SessionsDirBaseName = "sessions"
const MShellVersion = "0.1.0"
const RemoteIdFile = "remoteid"

var sessionDirCache = make(map[string]string)
var baseLock = &sync.Mutex{}

type CommandFileNames struct {
	PtyOutFile    string
	StdinFifo     string
	RunnerOutFile string
}

type CommandKey string

func MakeCommandKey(sessionId string, cmdId string) CommandKey {
	if sessionId == "" && cmdId == "" {
		return CommandKey("")
	}
	return CommandKey(fmt.Sprintf("%s/%s", sessionId, cmdId))
}

func (ckey CommandKey) IsEmpty() bool {
	return string(ckey) == ""
}

func (ckey CommandKey) GetSessionId() string {
	slashIdx := strings.Index(string(ckey), "/")
	if slashIdx == -1 {
		return ""
	}
	return string(ckey[0:slashIdx])
}

func (ckey CommandKey) GetCmdId() string {
	slashIdx := strings.Index(string(ckey), "/")
	if slashIdx == -1 {
		return ""
	}
	return string(ckey[slashIdx+1:])
}

func (ckey CommandKey) Split() (string, string) {
	fields := strings.SplitN(string(ckey), "/", 2)
	if len(fields) < 2 {
		return "", ""
	}
	return fields[0], fields[1]
}

func (ckey CommandKey) Validate(typeStr string) error {
	if typeStr == "" {
		typeStr = "ck"
	}
	if ckey == "" {
		return fmt.Errorf("%s has empty commandkey", typeStr)
	}
	sessionId, cmdId := ckey.Split()
	if sessionId == "" {
		return fmt.Errorf("%s does not have sessionid", typeStr)
	}
	_, err := uuid.Parse(sessionId)
	if err != nil {
		return fmt.Errorf("%s has invalid sessionid '%s'", typeStr, sessionId)
	}
	if cmdId == "" {
		return fmt.Errorf("%s does not have cmdid", typeStr)
	}
	_, err = uuid.Parse(cmdId)
	if err != nil {
		return fmt.Errorf("%s has invalid cmdid '%s'", typeStr, cmdId)
	}
	return nil
}

func GetHomeDir() string {
	homeVar := os.Getenv(HomeVarName)
	if homeVar == "" {
		return "/"
	}
	return homeVar
}

func GetMShellHomeDir() string {
	homeVar := os.Getenv(MShellHomeVarName)
	if homeVar != "" {
		return homeVar
	}
	return ExpandHomeDir(DefaultMShellHome)
}

func GetPtyOutFile(ck CommandKey, seqNum int) (string, error) {
	if err := ck.Validate("ck"); err != nil {
		return "", fmt.Errorf("cannot get command files: %w", err)
	}
	if seqNum < 0 {
		return "", fmt.Errorf("invalid seqnum, cannot be negative")
	}
	sessionId, cmdId := ck.Split()
	sdir, err := EnsureSessionDir(sessionId)
	if err != nil {
		return "", err
	}
	base := path.Join(sdir, cmdId)
	return fmt.Sprintf("%s.%d.ptyout", base, seqNum), nil
}

func GetCommandFileNames(ck CommandKey) (*CommandFileNames, error) {
	if err := ck.Validate("ck"); err != nil {
		return nil, fmt.Errorf("cannot get command files: %w", err)
	}
	sessionId, cmdId := ck.Split()
	sdir, err := EnsureSessionDir(sessionId)
	if err != nil {
		return nil, err
	}
	base := path.Join(sdir, cmdId)
	return &CommandFileNames{
		PtyOutFile:    base + ".ptyout",
		StdinFifo:     base + ".stdin",
		RunnerOutFile: base + ".runout",
	}, nil
}

func MakeCommandFileNamesWithHome(mhome string, ck CommandKey) *CommandFileNames {
	base := path.Join(mhome, SessionsDirBaseName, ck.GetSessionId(), ck.GetCmdId())
	return &CommandFileNames{
		PtyOutFile:    base + ".ptyout",
		StdinFifo:     base + ".stdin",
		RunnerOutFile: base + ".runout",
	}
}

func CleanUpCmdFiles(sessionId string, cmdId string) error {
	if cmdId == "" {
		return fmt.Errorf("bad cmdid, cannot clean up")
	}
	sdir, err := EnsureSessionDir(sessionId)
	if err != nil {
		return err
	}
	cmdFileGlob := path.Join(sdir, cmdId+".*")
	matches, err := filepath.Glob(cmdFileGlob)
	if err != nil {
		return err
	}
	for _, file := range matches {
		rmErr := os.Remove(file)
		if err == nil && rmErr != nil {
			err = rmErr
		}
	}
	return err
}

func EnsureSessionDir(sessionId string) (string, error) {
	if sessionId == "" {
		return "", fmt.Errorf("Bad sessionid, cannot be empty")
	}
	baseLock.Lock()
	sdir, ok := sessionDirCache[sessionId]
	baseLock.Unlock()
	if ok {
		return sdir, nil
	}
	mhome := GetMShellHomeDir()
	sdir = path.Join(mhome, SessionsDirBaseName, sessionId)
	info, err := os.Stat(sdir)
	if errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(sdir, 0777)
		if err != nil {
			return "", err
		}
		info, err = os.Stat(sdir)
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("session dir '%s' must be a directory", sdir)
	}
	baseLock.Lock()
	sessionDirCache[sessionId] = sdir
	baseLock.Unlock()
	return sdir, nil
}

func GetMShellPath() (string, error) {
	msPath := os.Getenv(MShellPathVarName) // use MSHELL_PATH
	if msPath != "" {
		return exec.LookPath(msPath)
	}
	mhome := GetMShellHomeDir()
	userMShellPath := path.Join(mhome, DefaultMShellName) // look in ~/.mshell
	msPath, err := exec.LookPath(userMShellPath)
	if err == nil {
		return msPath, nil
	}
	return exec.LookPath(DefaultMShellName) // standard path lookup for 'mshell'
}

func GetMShellSessionsDir() (string, error) {
	mhome := GetMShellHomeDir()
	return path.Join(mhome, SessionsDirBaseName), nil
}

func ExpandHomeDir(pathStr string) string {
	if pathStr != "~" && !strings.HasPrefix(pathStr, "~/") {
		return pathStr
	}
	homeDir := GetHomeDir()
	if pathStr == "~" {
		return homeDir
	}
	return path.Join(homeDir, pathStr[2:])
}

func ValidGoArch(goos string, goarch string) bool {
	return (goos == "darwin" || goos == "linux") && (goarch == "amd64" || goarch == "arm64")
}

func GoArchOptFile(goos string, goarch string) string {
	return fmt.Sprintf("/opt/mshell/bin/mshell.%s.%s", goos, goarch)
}

func GetRemoteId() (string, error) {
	mhome := GetMShellHomeDir()
	remoteIdFile := path.Join(mhome, RemoteIdFile)
	fd, err := os.Open(remoteIdFile)
	if errors.Is(err, fs.ErrNotExist) {
		// write the file
		remoteId := uuid.New().String()
		err = os.WriteFile(remoteIdFile, []byte(remoteId), 0644)
		if err != nil {
			return "", fmt.Errorf("cannot write remoteid to '%s': %w", remoteIdFile, err)
		}
		return remoteId, nil
	} else if err != nil {
		return "", fmt.Errorf("cannot read remoteid file '%s': %w", remoteIdFile, err)
	} else {
		defer fd.Close()
		contents, err := io.ReadAll(fd)
		if err != nil {
			return "", fmt.Errorf("cannot read remoteid file '%s': %w", remoteIdFile, err)
		}
		uuidStr := string(contents)
		_, err = uuid.Parse(uuidStr)
		if err != nil {
			return "", fmt.Errorf("invalid uuid read from '%s': %w", remoteIdFile, err)
		}
		return uuidStr, nil
	}
}
