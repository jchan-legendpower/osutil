// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Distro: Debian

package pkg

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/tredoe/osutil/v2"
	"github.com/tredoe/osutil/v2/config/shconf"
	"github.com/tredoe/osutil/v2/executil"
	"github.com/tredoe/osutil/v2/fileutil"
	"github.com/tredoe/osutil/v2/sysutil"
	"github.com/tredoe/osutil/v2/userutil"
)

// 'apt' is for the terminal and gives beautiful output.
// 'apt-get' and 'apt-cache' are for scripts and give stable, parsable output.

const (
	fileDeb = "apt-get"
	pathDeb = "/usr/bin/apt-get"

	//pathAptKey = "/usr/bin/apt-key"
	pathGpg = "/usr/bin/gpg"
)

// ManagerDeb is the interface to handle the package manager of Linux systems based at Debian.
type ManagerDeb struct {
	pathExec string
	cmd      *executil.Command
}

// NewManagerDeb returns the Deb package manager.
func NewManagerDeb() (ManagerDeb, error) {
	if err := userutil.MustBeSuperUser(sysutil.Linux); err != nil {
		return ManagerDeb{}, err
	}

	return ManagerDeb{
		pathExec: pathDeb,
		cmd: cmd.Command("", "").
			AddEnv([]string{"DEBIAN_FRONTEND=noninteractive"}).
			BadExitCodes([]int{100}),
	}, nil
}

func (m ManagerDeb) setPathExec(p string) { m.pathExec = p }

func (m ManagerDeb) Cmd() *executil.Command { return m.cmd }

func (m ManagerDeb) PackageType() string { return Deb.String() }

func (m ManagerDeb) PathExec() string { return m.pathExec }

func (m ManagerDeb) PreUsage() error {
	// == Directory required to import keys

	dirGnupg := "/root/.gnupg"

	info, err := os.Stat(dirGnupg)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return os.Mkdir(dirGnupg, 0700)
	}

	// If it already exists
	if !info.IsDir() {
		return fmt.Errorf("%q must be a directory", dirGnupg)
	}

	return nil
}

func (m ManagerDeb) SetStdout(out io.Writer) { m.cmd.Stdout(out) }

// * * *

func (m ManagerDeb) Install(name ...string) error {
	osutil.Log.Print(taskInstall)
	args := append([]string{"install", "-y"}, name...)

	_, err := m.cmd.Command(pathDeb, args...).Run()
	return err
}

func (m ManagerDeb) Remove(name ...string) error {
	osutil.Log.Print(taskRemove)
	args := append([]string{"remove", "-y"}, name...)

	_, err := m.cmd.Command(pathDeb, args...).Run()
	return err
}

func (m ManagerDeb) Purge(name ...string) error {
	osutil.Log.Print(taskPurge)
	args := append([]string{"purge", "-y"}, name...)

	_, err := m.cmd.Command(pathDeb, args...).Run()
	return err
}

func (m ManagerDeb) UpdateIndex() error {
	osutil.Log.Print(taskUpdate)
	stderr, err := m.cmd.Command(pathDeb, "update", "-qq").OutputStderr()

	return executil.CheckStderr(stderr, err)
}

func (m ManagerDeb) Update() error {
	osutil.Log.Print(taskUpgrade)
	_, err := m.cmd.Command(pathDeb, "upgrade", "-y").Run()
	return err
}

func (m ManagerDeb) Clean() error {
	osutil.Log.Print(taskClean)
	_, err := m.cmd.Command(pathDeb, "autoremove", "-y").Run()
	if err != nil {
		return err
	}

	_, err = m.cmd.Command(pathDeb, "clean").Run()
	return err
}

// https://www.linuxuprising.com/2021/01/apt-key-is-deprecated-how-to-add.html

func (m ManagerDeb) ImportKey(alias, keyUrl string) error {
	osutil.Log.Print(taskImportKey)
	if file := path.Base(keyUrl); !strings.Contains(file, ".") {
		return ErrKeyUrl
	}

	var keyFile bytes.Buffer

	err := fileutil.Dload(keyUrl, &keyFile)
	if err != nil {
		return err
	}

	stdout, stderr, err := m.cmd.Command(
		pathGpg, "--dearmor", keyFile.String(),
	).OutputCombined()
	if err = executil.CheckStderr(stderr, err); err != nil {
		return err
	}

	fi, err := os.Create(m.keyring(alias))
	if err != nil {
		return err
	}
	defer func() {
		if err2 := fi.Close(); err2 != nil && err == nil {
			err = err2
		}
	}()
	if _, err = fi.Write(stdout); err != nil {
		return err
	}

	return fi.Sync()
}

func (m ManagerDeb) ImportKeyFromServer(alias, keyServer, key string) error {
	osutil.Log.Print(taskImportKeyFromServer)
	if keyServer == "" {
		keyServer = "hkp://keyserver.ubuntu.com:80"
	}

	/*stderr, err := m.cmd.Command(
		pathAptKey,
		"adv",
		"--keyserver", keyServer,
		"--recv", key,
	).OutputStderr()*/

	stderr, err := m.cmd.Command(
		pathGpg,
		"--no-default-keyring",
		"--keyring", m.keyring(alias),
		"--keyserver", keyServer,
		"--recv-keys", key,
	).OutputStderr()

	if bytes.Contains(stderr, failedB) {
		return fmt.Errorf("%s", stderr)
	}
	return err
}

func (m ManagerDeb) RemoveKey(alias string) error {
	osutil.Log.Print(taskRemoveKey)
	return os.Remove(m.keyring(alias))
}

func (m ManagerDeb) AddRepo(alias string, url ...string) (err error) {
	osutil.Log.Print(taskAddRepo)
	//distroName, err := distroCodeName()
	//if err != nil {
	//	return err
	//}

	// If there is an error like:
	// E: The repository 'https://repo... focal Release' does not have a Release file.
	//
	// Then, there is to use:
	// "deb [signed-by=%s] %s main/""

	err = fileutil.CreateFromString(
		m.repository(alias),
		//fmt.Sprintf("deb [signed-by=%s] %s %s main\n",
		//	m.keyring(alias), url[0], distroName,
		//),
		fmt.Sprintf("deb [signed-by=%s] %s main/\n",
			m.keyring(alias), url[0],
		),
	)
	if err != nil {
		return err
	}

	return m.UpdateIndex()
}

func (m ManagerDeb) RemoveRepo(alias string) error {
	osutil.Log.Print(taskRemoveRepo)
	err := os.Remove(m.keyring(alias))
	if err != nil {
		return err
	}
	if err = os.Remove(m.repository(alias)); err != nil {
		return err
	}

	return m.UpdateIndex()
}

// == Utility
//

// distroCodeName returns the version like code name.
func distroCodeName() (string, error) {
	_, err := os.Stat("/etc/os-release")
	if os.IsNotExist(err) {
		return "", fmt.Errorf("%s", sysutil.DistroUnknown)
	}

	cfg, err := shconf.ParseFile("/etc/os-release")
	if err != nil {
		return "", err
	}

	return cfg.Get("VERSION_CODENAME")
}

func (m ManagerDeb) keyring(alias string) string {
	return "/usr/share/keyrings/" + alias + "-archive-keyring.gpg"
}

func (m ManagerDeb) repository(alias string) string {
	return "/etc/apt/sources.list.d/" + alias + ".list"
}
