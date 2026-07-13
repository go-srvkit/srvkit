// Copyright © 2023 Johnnie Chen. All rights reserved.
// See accompanying LICENSE file.

package binver

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	_Version   = "v0.0.0" // 版本号
	_GitBranch string     // 代码分支
	_CommitRev string     // 提交版本
	_BuildTime string     // 编译时间
)

func GitBranch() string {
	return _GitBranch
}

func CommitRev() string {
	return _CommitRev
}

func BuildTime() string {
	return _BuildTime
}

func BinVersion() string {
	return fmt.Sprintf("%s %s.%s built at %s powered by %s", _Version, _GitBranch, _CommitRev,
		_BuildTime, runtime.Version())
}

func String() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Version: %s\n", _Version)
	fmt.Fprintf(&sb, "Revision: %s-%s\n", _GitBranch, _CommitRev)
	fmt.Fprintf(&sb, "Built at: %s\n", _BuildTime)
	fmt.Fprintf(&sb, "Powered by: %s", runtime.Version())
	return sb.String()
}
