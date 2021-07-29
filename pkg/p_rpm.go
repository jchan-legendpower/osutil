// Copyright 2012 Jonas mg
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import "github.com/tredoe/osutil/sh"

type rpm struct{}

func (p rpm) Install(name ...string) error {
	args := []string{"install"}

	return sh.ExecToStd(nil, "/usr/bin/yum", append(args, name...)...)
}

func (p rpm) Remove(name ...string) error {
	args := []string{"remove"}

	return sh.ExecToStd(nil, "/usr/bin/yum", append(args, name...)...)
}

func (p rpm) Purge(name ...string) error {
	return p.Remove(name...)
}

func (p rpm) Update() error {
	return sh.ExecToStd(nil, "/usr/bin/yum", "update")
}

func (p rpm) Upgrade() error {
	return sh.ExecToStd(nil, "/usr/bin/yum", "update")
}

func (p rpm) Clean() error {
	return sh.ExecToStd(nil, "/usr/bin/yum", "clean", "packages")
}
