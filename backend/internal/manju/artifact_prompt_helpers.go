package manju

import (
	"fmt"
	"strings"
)

func appendLockedArtifactAuthority(out *strings.Builder, upstream string) {
	out.WriteString("【系统门禁状态具有最终权威】\n")
	fmt.Fprintf(
		out,
		"下方 %s 工件已由系统记录为用户确认并处于 LOCKED 状态。工件正文中的工作稿/未审核/未锁定只是生成时自述，不代表当前门禁状态，不得据此输出门禁阻断单、拒绝生成本阶段内容或要求重新锁定。\n\n",
		strings.TrimSpace(upstream),
	)
}

func markdownSection(content, heading string) string {
	start := strings.Index(content, heading)
	if start < 0 {
		return ""
	}
	section := content[start+len(heading):]
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	return section
}
